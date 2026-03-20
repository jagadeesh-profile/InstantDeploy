package runtime

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"instantdeploy/backend/pkg/models"

	"github.com/redis/go-redis/v9"
)

const (
	idleCleanupInterval = 1 * time.Minute
	idleDeploymentTTL   = 20 * time.Minute
	defaultBuildTimeout = 8 * time.Minute
	defaultBuildRetries = 1
	defaultBuildWorkers = 3
	defaultQueueSize    = 256
	defaultQueueKey     = "instantdeploy:build_queue"
	defaultExecutionMode = "docker"
)

// RuntimeEvent is published to subscribers on status changes and log lines.
type RuntimeEvent struct {
	Type         string    `json:"type"`
	DeploymentID string    `json:"deploymentId"`
	UserID       string    `json:"userId,omitempty"`
	Status       string    `json:"status,omitempty"`
	Level        string    `json:"level,omitempty"`
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

type buildRequest struct {
	deploymentID string
	repoURL      string
	displayRepo  string
	branch       string
	customURL    string
}

type buildQueueMessage struct {
	DeploymentID string `json:"deployment_id"`
	RepoURL      string `json:"repo_url"`
	DisplayRepo  string `json:"display_repo"`
	Branch       string `json:"branch"`
	CustomURL    string `json:"custom_url"`
}

// Manager is the main runtime controller: creates, builds, monitors, and deletes deployments.
type Manager struct {
	mu           sync.RWMutex
	deployments  []models.Deployment
	logs         map[string][]models.DeploymentLog
	lastTouched  map[string]time.Time
	buildTimeout time.Duration
	buildRetries int
	workerCount  int
	buildQueue   chan buildRequest
	redisQueue   *redis.Client
	queueKey     string
	subscribers  map[int]chan RuntimeEvent
	nextSubID    int
	store        Store
}

func NewManager() *Manager {
	return NewManagerWithStoreAndQueue(nil, nil)
}

func NewManagerWithStore(store Store) *Manager {
	return NewManagerWithStoreAndQueue(store, nil)
}

func NewManagerWithStoreAndQueue(store Store, redisQueue *redis.Client) *Manager {
	workerCount := getBuildWorkersFromEnv()
	m := &Manager{
		deployments:  make([]models.Deployment, 0),
		logs:         make(map[string][]models.DeploymentLog),
		lastTouched:  make(map[string]time.Time),
		buildTimeout: getBuildTimeoutFromEnv(),
		buildRetries: getBuildRetriesFromEnv(),
		workerCount:  workerCount,
		buildQueue:   make(chan buildRequest, defaultQueueSize),
		redisQueue:   redisQueue,
		queueKey:     getQueueKeyFromEnv(),
		subscribers:  make(map[int]chan RuntimeEvent),
		store:        store,
	}
	m.hydrateFromStore()
	m.cleanupOrphanedManagedContainers()
	for i := 0; i < workerCount; i++ {
		go m.workerLoop(i + 1)
	}
	go m.cleanupInactiveLoop()
	return m
}

// Stats holds runtime queue/deployment statistics.
type Stats struct {
	QueueMode   string `json:"queueMode"`
	QueueDepth  int64  `json:"queueDepth"`
	Workers     int    `json:"workers"`
	Deployments int    `json:"deployments"`
	Logs        int    `json:"logs"`
}

func (m *Manager) QueueMode() string {
	if m.redisQueue != nil {
		return "redis"
	}
	return "memory"
}

func (m *Manager) Stats() (Stats, error) {
	stats := Stats{
		QueueMode: m.QueueMode(),
		Workers:   m.workerCount,
	}
	m.mu.RLock()
	stats.Deployments = len(m.deployments)
	for _, items := range m.logs {
		stats.Logs += len(items)
	}
	if m.redisQueue == nil {
		stats.QueueDepth = int64(len(m.buildQueue))
		m.mu.RUnlock()
		return stats, nil
	}
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	depth, err := m.redisQueue.LLen(ctx, m.queueKey).Result()
	if err != nil {
		return stats, fmt.Errorf("failed to read redis queue depth: %w", err)
	}
	stats.QueueDepth = depth
	return stats, nil
}

func (m *Manager) hydrateFromStore() {
	if m.store == nil {
		return
	}
	if err := m.store.EnsureSchema(); err != nil {
		log.Printf("runtime persistence schema setup failed: %v", err)
		return
	}
	deployments, err := m.store.ListDeployments()
	if err != nil {
		log.Printf("runtime persistence load deployments failed: %v", err)
		return
	}
	logsByDeployment, err := m.store.ListLogsByDeployment()
	if err != nil {
		log.Printf("runtime persistence load logs failed: %v", err)
		return
	}

	m.deployments = deployments
	m.logs = logsByDeployment
	now := time.Now().UTC()
	usingRedis := m.redisQueue != nil

	for i := range deployments {
		id := deployments[i].ID
		if _, ok := m.logs[id]; !ok {
			m.logs[id] = []models.DeploymentLog{}
		}
		m.lastTouched[id] = now

		status := deployments[i].Status
		isStuck := status == "cloning" || status == "building" || status == "starting" ||
			(status == "queued" && !usingRedis)
		if isStuck {
			m.deployments[i].Status = "failed"
			m.deployments[i].Error = "Server restarted before this deployment could complete"
			if m.store != nil {
				snapshot := m.deployments[i]
				go func() {
					if err := m.store.UpsertDeployment(snapshot); err != nil {
						log.Printf("runtime persistence: could not fail interrupted deployment %s: %v", snapshot.ID, err)
					}
				}()
			}
		}
	}
}

func (m *Manager) Create(repositoryInput, branch, customURL, requestedBy string) (models.Deployment, error) {
	repoURL, displayRepo, err := normalizeRepositoryInput(repositoryInput)
	if err != nil {
		return models.Deployment{}, err
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	id := fmt.Sprintf("dep_%d", time.Now().UnixNano())
	deployment := models.Deployment{
		ID:         id,
		UserID:     strings.TrimSpace(requestedBy),
		Repository: displayRepo,
		RepoURL:    repoURL,
		Branch:     branch,
		Status:     "queued",
		URL:        strings.TrimSpace(customURL),
		CreatedAt:  time.Now().UTC(),
	}

	m.mu.Lock()
	m.deployments = append([]models.Deployment{deployment}, m.deployments...)
	m.logs[deployment.ID] = []models.DeploymentLog{}
	m.lastTouched[deployment.ID] = time.Now().UTC()
	m.persistDeploymentLocked(deployment.ID)
	m.appendLogLocked(deployment.ID, "info", "Deployment request accepted")
	m.appendLogLocked(deployment.ID, "info", "Deployment queued")
	m.mu.Unlock()

	if !m.enqueueBuild(buildRequest{
		deploymentID: id,
		repoURL:      repoURL,
		displayRepo:  displayRepo,
		branch:       branch,
		customURL:    customURL,
	}) {
		m.markFailed(id, "build queue is full, try again in a few moments")
	}
	return deployment, nil
}

func (m *Manager) Subscribe(buffer int) (int, <-chan RuntimeEvent) {
	if buffer <= 0 {
		buffer = 64
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextSubID++
	id := m.nextSubID
	ch := make(chan RuntimeEvent, buffer)
	m.subscribers[id] = ch
	return id, ch
}

func (m *Manager) Unsubscribe(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ch, ok := m.subscribers[id]; ok {
		delete(m.subscribers, id)
		close(ch)
	}
}

func (m *Manager) List() []models.Deployment {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for i := range m.deployments {
		m.lastTouched[m.deployments[i].ID] = now
	}
	out := make([]models.Deployment, len(m.deployments))
	copy(out, m.deployments)
	return out
}

func (m *Manager) ListByUser(userID string) []models.Deployment {
	if strings.TrimSpace(userID) == "" {
		return []models.Deployment{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	out := make([]models.Deployment, 0, len(m.deployments))
	for i := range m.deployments {
		if m.deployments[i].UserID != userID {
			continue
		}
		m.lastTouched[m.deployments[i].ID] = now
		out = append(out, m.deployments[i])
	}
	return out
}

func (m *Manager) Get(deploymentID string) (models.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			m.lastTouched[deploymentID] = time.Now().UTC()
			return m.deployments[i], nil
		}
	}
	return models.Deployment{}, errors.New("deployment not found")
}

func (m *Manager) GetForUser(deploymentID, userID string) (models.Deployment, error) {
	if strings.TrimSpace(userID) == "" {
		return models.Deployment{}, errors.New("deployment not found")
	}
	dep, err := m.Get(deploymentID)
	if err != nil {
		return models.Deployment{}, err
	}
	if dep.UserID != userID {
		return models.Deployment{}, errors.New("deployment not found")
	}
	return dep, nil
}

func (m *Manager) Logs(deploymentID string) ([]models.DeploymentLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs, ok := m.logs[deploymentID]
	if !ok {
		return nil, errors.New("deployment not found")
	}
	m.lastTouched[deploymentID] = time.Now().UTC()
	out := make([]models.DeploymentLog, len(logs))
	copy(out, logs)
	return out, nil
}

func (m *Manager) LogsForUser(deploymentID, userID string) ([]models.DeploymentLog, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("deployment not found")
	}
	dep, err := m.Get(deploymentID)
	if err != nil {
		return nil, err
	}
	if dep.UserID != userID {
		return nil, errors.New("deployment not found")
	}
	return m.Logs(deploymentID)
}

func (m *Manager) Delete(deploymentID string) error {
	m.mu.Lock()
	idx := -1
	var dep models.Deployment
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			idx = i
			dep = m.deployments[i]
			break
		}
	}
	if idx == -1 {
		m.mu.Unlock()
		return errors.New("deployment not found")
	}
	m.appendLogLocked(deploymentID, "info", "Delete requested")
	m.mu.Unlock()

	if dep.Container != "" {
		if strings.HasPrefix(dep.Container, "k8s:") {
			deleteKubernetesDeployment(strings.TrimPrefix(dep.Container, "k8s:"))
		} else {
			_ = runCmd(context.Background(), "docker", "rm", "-v", "-f", dep.Container)
		}
	}
	if dep.Image != "" {
		_ = runCmd(context.Background(), "docker", "rmi", "-f", dep.Image)
		// Launch background cleanup to reclaim build cache space and prevent disk bloat
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			_ = runCmd(ctx, "docker", "image", "prune", "-f")
			_ = runCmd(ctx, "docker", "builder", "prune", "-f")
			_ = runCmd(ctx, "docker", "volume", "prune", "-f")
		}()
	}


	m.mu.Lock()
	defer m.mu.Unlock()
	idx = -1
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		m.deployments = append(m.deployments[:idx], m.deployments[idx+1:]...)
	}
	delete(m.logs, deploymentID)
	delete(m.lastTouched, deploymentID)
	go m.persistDelete(deploymentID)
	return nil
}

func (m *Manager) DeleteForUser(deploymentID, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("deployment not found")
	}
	dep, err := m.Get(deploymentID)
	if err != nil {
		return err
	}
	if dep.UserID != userID {
		return errors.New("deployment not found")
	}
	return m.Delete(deploymentID)
}

func (m *Manager) cleanupInactiveLoop() {
	ticker := time.NewTicker(idleCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.cleanupInactiveDeployments()
	}
}

func (m *Manager) cleanupInactiveDeployments() {
	cutoff := time.Now().UTC().Add(-idleDeploymentTTL)
	var staleIDs []string

	m.mu.RLock()
	for i := range m.deployments {
		dep := m.deployments[i]
		if dep.Status != "running" && dep.Status != "failed" {
			continue
		}
		last, ok := m.lastTouched[dep.ID]
		if !ok {
			last = dep.CreatedAt
		}
		if last.Before(cutoff) {
			staleIDs = append(staleIDs, dep.ID)
		}
	}
	m.mu.RUnlock()

	for _, id := range staleIDs {
		_ = m.Delete(id)
	}
	m.cleanupExitedManagedContainers()
}

func (m *Manager) cleanupOrphanedManagedContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := runCmdOutput(ctx, "docker", "ps", "-a", "--filter", "label=instantdeploy.managed=true", "--format", "{{.ID}} {{.Label \"instantdeploy.deployment_id\"}}")
	if err != nil {
		return
	}
	tracked := make(map[string]struct{}, len(m.deployments))
	m.mu.RLock()
	for i := range m.deployments {
		tracked[m.deployments[i].ID] = struct{}{}
	}
	m.mu.RUnlock()

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		containerID := parts[0]
		deploymentID := parts[1]
		if _, ok := tracked[deploymentID]; ok {
			continue
		}
		_ = runCmd(context.Background(), "docker", "rm", "-f", containerID)
	}
}

func (m *Manager) cleanupExitedManagedContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = runCmd(ctx, "docker", "container", "prune", "--force", "--filter", "label=instantdeploy.managed=true")
}

// buildAndRun is the core build pipeline: clone → detect → fix → generate Dockerfile → docker build → docker run.
func (m *Manager) buildAndRun(deploymentID, repoURL, displayRepo, branch, customURL string) {
	if !m.deploymentExists(deploymentID) {
		return
	}

	m.updateStatus(deploymentID, "cloning")
	m.appendLog(deploymentID, "info", "Cloning repository")

	tmpDir, err := os.MkdirTemp("", "instantdeploy-*")
	if err != nil {
		m.markFailed(deploymentID, fmt.Sprintf("failed to create temp dir: %v", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	ctxClone, cancelClone := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelClone()
	if err := runCmd(ctxClone, "git", "clone", "--depth", "1", "--branch", branch, repoURL, tmpDir); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Clone with branch %q failed, trying default branch", branch))
		ctxFallback, cancelFallback := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancelFallback()
		if errFallback := runCmd(ctxFallback, "git", "clone", "--depth", "1", repoURL, tmpDir); errFallback != nil {
			m.markFailed(deploymentID, fmt.Sprintf("git clone failed: %v", err))
			return
		}
	}
	m.appendLog(deploymentID, "info", "Repository cloned")

	// Detect, fix, and generate Dockerfile — all fixes applied automatically
	projectDir, dockerfilePath, containerPort, err := m.ensureDockerfile(tmpDir, deploymentID)
	if err != nil {
		m.markFailed(deploymentID, err.Error())
		return
	}
	if projectDir != tmpDir {
		m.appendLog(deploymentID, "info", fmt.Sprintf("Using project root: %s", projectDir))
	}
	m.appendLog(deploymentID, "info", fmt.Sprintf("Using Dockerfile: %s (container port: %d)", filepath.Base(dockerfilePath), containerPort))

	image := sanitizeName("instantdeploy-" + deploymentID)
	container := sanitizeName("instantdeploy-" + deploymentID)

	hostPort, err := findAvailablePort(20000, 39999)
	if err != nil {
		m.markFailed(deploymentID, fmt.Sprintf("failed to allocate host port: %v", err))
		return
	}
	m.appendLog(deploymentID, "info", fmt.Sprintf("Allocated host port %d", hostPort))

	m.updateStatus(deploymentID, "building")
	m.appendLog(deploymentID, "info", "Checking Docker daemon")
	if dockerErr := checkDockerDaemonAvailable(); dockerErr != nil {
		m.markFailed(deploymentID, fmt.Sprintf("docker unavailable: %v", dockerErr))
		return
	}

	m.appendLog(deploymentID, "info", "Building Docker image")
	buildErr := m.runDockerBuildWithRetries(projectDir, image, dockerfilePath, deploymentID)
	if buildErr != nil {
		// Try patching deprecated base images
		patched, replacement, patchErr := patchDeprecatedDockerBaseImage(dockerfilePath, buildErr.Error())
		if patchErr == nil && patched {
			m.appendLog(deploymentID, "warn", fmt.Sprintf("Patched deprecated base image to %s, retrying", replacement))
			buildErr = m.runDockerBuildWithRetries(projectDir, image, dockerfilePath, deploymentID)
		}

		// Try Java fallback
		if buildErr != nil && shouldUseJavaDockerFallback(projectDir, buildErr.Error()) {
			fallbackDockerfile, ok, fallbackErr := writeJavaDockerfile(projectDir)
			if fallbackErr == nil && ok {
				dockerfilePath = fallbackDockerfile
				containerPort = 8080
				m.appendLog(deploymentID, "warn", "Using Java fallback Dockerfile")
				buildErr = m.runDockerBuildWithRetries(projectDir, image, dockerfilePath, deploymentID)
			}
		}

		// One-time simple fallback requested in execution plan:
		// detect language from top-level files and generate a minimal Dockerfile.
		if buildErr != nil {
			simpleLang := detectSimpleLanguage(projectDir)
			if simpleLang != "unknown" {
				simpleDockerfilePath, simplePort, simpleErr := writeSimpleDockerfile(projectDir, simpleLang)
				if simpleErr == nil {
					dockerfilePath = simpleDockerfilePath
					if simplePort > 0 {
						containerPort = simplePort
					}
					m.appendLog(deploymentID, "warn", fmt.Sprintf("Using one-time simple fallback Dockerfile for %s", simpleLang))
					buildErr = m.runDockerBuildWithRetries(projectDir, image, dockerfilePath, deploymentID)
				}
			}
		}

		if buildErr != nil {
			m.markFailed(deploymentID, fmt.Sprintf("docker build failed: %v", buildErr))
			return
		}
	}
	m.appendLog(deploymentID, "info", "Docker image built")

	_ = runCmd(context.Background(), "docker", "rm", "-f", container)

	mode := getExecutionModeFromEnv()
	localURL := fmt.Sprintf("http://localhost:%d", hostPort)
	resourceRef := container
	if mode == "kubernetes" {
		m.updateStatus(deploymentID, "starting")
		m.appendLog(deploymentID, "info", "Starting Kubernetes deployment")
		k8sName := sanitizeName("idep-" + deploymentID)
		if err := deployToKubernetes(k8sName, image, containerPort, hostPort); err != nil {
			m.markFailed(deploymentID, fmt.Sprintf("kubernetes deploy failed: %v", err))
			return
		}
		resourceRef = "k8s:" + k8sName
		if readyErr := waitForAppReady(hostPort, 60*time.Second); readyErr != nil {
			m.markFailed(deploymentID, fmt.Sprintf("kubernetes app failed to become reachable: %v", readyErr))
			return
		}
	} else {
		m.updateStatus(deploymentID, "starting")
		m.appendLog(deploymentID, "info", "Starting container")
		simpleFallbackTried := false
		ctxRun, cancelRun := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancelRun()
		if err := runCmd(ctxRun, "docker", dockerRunArgs(container, image, deploymentID, hostPort, containerPort)...); err != nil {
			m.appendLog(deploymentID, "warn", fmt.Sprintf("Container start failed: %v", err))
			if !simpleFallbackTried {
				fallbackPort, ok := m.trySimpleFallbackDeployment(projectDir, deploymentID, image, container, hostPort)
				simpleFallbackTried = true
				if ok {
					containerPort = fallbackPort
				} else {
					m.markFailed(deploymentID, fmt.Sprintf("docker run failed: %v", err))
					return
				}
			} else {
				m.markFailed(deploymentID, fmt.Sprintf("docker run failed: %v", err))
				return
			}
		}

		m.appendLog(deploymentID, "info", "Waiting for application readiness")
		if readyErr := waitForContainerReady(container, hostPort, containerPort, 60*time.Second); readyErr != nil {
			if strings.Contains(strings.ToLower(readyErr.Error()), "timeout after") {
				containerLogs := getContainerLogs(container)
				if detectedPort, ok := m.tryPortHintDeployment(projectDir, deploymentID, image, container, hostPort, containerPort, containerLogs); ok {
					containerPort = detectedPort
					readyErr = nil
				}
			}

			if readyErr == nil {
				// Recovered by retrying container startup with a detected listening port.
			} else if !simpleFallbackTried && strings.Contains(strings.ToLower(readyErr.Error()), "status \"exited\"") {
				m.appendLog(deploymentID, "warn", fmt.Sprintf("Container exited during readiness: %v", readyErr))
				fallbackPort, ok := m.trySimpleFallbackDeployment(projectDir, deploymentID, image, container, hostPort)
				simpleFallbackTried = true
				if ok {
					containerPort = fallbackPort
				} else {
					containerLogs := getContainerLogs(container)
					if containerLogs != "" {
						m.appendLog(deploymentID, "error", containerLogs)
					}
					m.markFailed(deploymentID, fmt.Sprintf("app failed to become reachable: %v", readyErr))
					return
				}
			} else {
				containerLogs := getContainerLogs(container)
				if containerLogs != "" {
					m.appendLog(deploymentID, "error", containerLogs)
				}
				errMsg := fmt.Sprintf("app failed to become reachable: %v", readyErr)
				if hasLoopbackBindHint(containerLogs) {
					errMsg += " — app appears to bind to 127.0.0.1/localhost inside container; bind to 0.0.0.0"
				}
				m.markFailed(deploymentID, errMsg)
				return
			}
		}
	}

	finalURL := strings.TrimSpace(customURL)
	if finalURL == "" {
		finalURL = localURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			m.deployments[i].Status = "running"
			m.deployments[i].URL = finalURL
			m.deployments[i].LocalURL = localURL
			m.deployments[i].Image = image
			m.deployments[i].Container = resourceRef
			m.deployments[i].Repository = displayRepo
			m.publishLocked(RuntimeEvent{
				Type:         "status",
				DeploymentID: deploymentID,
				UserID:       m.deployments[i].UserID,
				Status:       "running",
				Timestamp:    time.Now().UTC(),
			})
			m.persistDeploymentLocked(deploymentID)
			m.appendLogLocked(deploymentID, "info", fmt.Sprintf("Deployment is live at %s", localURL))
			return
		}
	}
}

func (m *Manager) trySimpleFallbackDeployment(projectDir, deploymentID, image, container string, hostPort int) (int, bool) {
	lang := detectSimpleLanguage(projectDir)
	if lang == "unknown" {
		m.appendLog(deploymentID, "warn", "Simple fallback skipped: unknown language")
		return 0, false
	}

	dockerfilePath, fallbackPort, err := writeSimpleDockerfile(projectDir, lang)
	if err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Simple fallback Dockerfile generation failed: %v", err))
		return 0, false
	}

	m.appendLog(deploymentID, "warn", fmt.Sprintf("Retrying with simple fallback Dockerfile for %s", lang))
	if err := m.runDockerBuildWithRetries(projectDir, image, dockerfilePath, deploymentID); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Simple fallback build failed: %v", err))
		return 0, false
	}

	_ = runCmd(context.Background(), "docker", "rm", "-f", container)
	ctxRun, cancelRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelRun()
	if err := runCmd(ctxRun, "docker", dockerRunArgs(container, image, deploymentID, hostPort, fallbackPort)...); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Simple fallback run failed: %v", err))
		return 0, false
	}

	if err := waitForContainerReady(container, hostPort, fallbackPort, 60*time.Second); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Simple fallback readiness failed: %v", err))
		return 0, false
	}

	return fallbackPort, true
}

func (m *Manager) tryPortHintDeployment(projectDir, deploymentID, image, container string, hostPort, currentPort int, containerLogs string) (int, bool) {
	inferredPort := inferPortFromContainerLogs(containerLogs)
	if inferredPort == 0 || inferredPort == currentPort {
		return 0, false
	}

	m.appendLog(deploymentID, "warn", fmt.Sprintf("Detected app port %d from logs; retrying container startup", inferredPort))

	_ = runCmd(context.Background(), "docker", "rm", "-f", container)
	ctxRun, cancelRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelRun()
	if err := runCmd(ctxRun, "docker", dockerRunArgs(container, image, deploymentID, hostPort, inferredPort)...); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Port-hint retry run failed: %v", err))
		return 0, false
	}

	if err := waitForContainerReady(container, hostPort, inferredPort, 60*time.Second); err != nil {
		m.appendLog(deploymentID, "warn", fmt.Sprintf("Port-hint retry readiness failed: %v", err))
		return 0, false
	}

	return inferredPort, true
}

// ensureDockerfile runs SmartDetector → BuildFixer → DockerfileGenerator,
// detecting the project type & applying any needed fixes.
func (m *Manager) ensureDockerfile(repoDir, deploymentID string) (projectDir, dockerfilePath string, containerPort int, err error) {
	logf := func(level, message string) {
		m.appendLog(deploymentID, level, message)
	}

	// 0. Check if the project files are in a subdirectory
	repoDir = findProjectRoot(repoDir, logf)

	// 1. Detect project type
	detector := NewSmartDetector()
	project, err := detector.Detect(repoDir)
	if err != nil {
		return "", "", 0, fmt.Errorf("project detection failed: %w", err)
	}
	logf("info", fmt.Sprintf("Detected project: %s", project.Summary))

	if len(project.SkipPlugins) > 0 {
		logf("warn", fmt.Sprintf("Found problematic plugins: %v — applying fixes", project.SkipPlugins))
	}

	// 1b. Validate required files exist
	validateProjectFiles(repoDir, project, logf)

	// 2. Fix build files if needed
	if project.FixRequired || len(project.SkipPlugins) > 0 {
		logf("info", "Applying build fixes...")
		fixer := NewBuildFixer(logf)
		if fixErr := fixer.Fix(repoDir, project); fixErr != nil {
			logf("warn", fmt.Sprintf("Build fix warning (continuing): %v", fixErr))
		} else {
			logf("info", "Build files fixed")
		}
	}

	// 3. Generate Dockerfile
	generator := NewDockerfileGenerator(logf)
	dockerfilePath, containerPort, err = generator.Generate(repoDir, project)
	if err != nil {
		return "", "", 0, err
	}
	return repoDir, dockerfilePath, containerPort, nil
}

// findProjectRoot checks if the project files are in the root or a subdirectory.
// For monorepos or example repositories, the actual project may be nested.
func findProjectRoot(repoDir string, logf func(string, string)) string {
	// Project indicators — if any of these exist at root, it's fine
	indicators := []string{
		"package.json", "Dockerfile", "pom.xml", "build.gradle", "build.gradle.kts",
		"go.mod", "Cargo.toml", "requirements.txt", "Pipfile", "pyproject.toml",
		"Gemfile", "composer.json", "index.html",
	}
	for _, f := range indicators {
		if fileExists(filepath.Join(repoDir, f)) {
			return repoDir // Project is at root
		}
	}

	// Not at root — scan one level of subdirectories
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return repoDir
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subDir := filepath.Join(repoDir, entry.Name())
		for _, f := range indicators {
			if fileExists(filepath.Join(subDir, f)) {
				logf("info", fmt.Sprintf("Project found in subdirectory: %s", entry.Name()))
				return subDir
			}
		}
	}

	return repoDir
}

// validateProjectFiles checks that required files exist for the detected type
// and logs warnings if they're missing.
func validateProjectFiles(repoDir string, cfg *ProjectConfig, logf func(string, string)) {
	var missing []string

	switch {
	case strings.HasPrefix(cfg.Type, "node") || cfg.Type == "node-static":
		if !fileExists(filepath.Join(repoDir, "package.json")) {
			missing = append(missing, "package.json")
		}
	case strings.HasPrefix(cfg.Type, "python"):
		// Check for at least one of the dependency files
		hasDepFile := fileExists(filepath.Join(repoDir, "requirements.txt")) ||
			fileExists(filepath.Join(repoDir, "Pipfile")) ||
			fileExists(filepath.Join(repoDir, "pyproject.toml"))
		if !hasDepFile {
			logf("warn", "No requirements.txt, Pipfile, or pyproject.toml found")
		}
		// Check for Python entrypoint
		hasPyEntry := false
		for _, name := range []string{"app.py", "main.py", "server.py", "manage.py", "wsgi.py"} {
			if fileExists(filepath.Join(repoDir, name)) {
				hasPyEntry = true
				break
			}
		}
		if !hasPyEntry {
			logf("warn", "No standard Python entrypoint found (app.py, main.py, server.py)")
		}
	case strings.HasPrefix(cfg.Type, "java"):
		if !fileExists(filepath.Join(repoDir, "pom.xml")) &&
			!fileExists(filepath.Join(repoDir, "build.gradle")) &&
			!fileExists(filepath.Join(repoDir, "build.gradle.kts")) {
			missing = append(missing, "pom.xml or build.gradle")
		}
	case cfg.Type == "static":
		if cfg.OutputDir == "" {
			logf("warn", "No index.html found in repository")
		}
	}

	for _, f := range missing {
		logf("warn", fmt.Sprintf("Missing required file: %s", f))
	}
}

func (m *Manager) enqueueBuild(req buildRequest) bool {
	if m.redisQueue != nil {
		payload, err := json.Marshal(buildQueueMessage{
			DeploymentID: req.deploymentID,
			RepoURL:      req.repoURL,
			DisplayRepo:  req.displayRepo,
			Branch:       req.branch,
			CustomURL:    req.customURL,
		})
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return m.redisQueue.LPush(ctx, m.queueKey, payload).Err() == nil
	}

	select {
	case m.buildQueue <- req:
		return true
	default:
		return false
	}
}

func (m *Manager) workerLoop(workerID int) {
	if m.redisQueue != nil {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			result, err := m.redisQueue.BRPop(ctx, 3*time.Second, m.queueKey).Result()
			cancel()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, redis.Nil) {
					continue
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if len(result) != 2 {
				continue
			}
			var msg buildQueueMessage
			if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
				continue
			}
			req := buildRequest{
				deploymentID: msg.DeploymentID,
				repoURL:      msg.RepoURL,
				displayRepo:  msg.DisplayRepo,
				branch:       msg.Branch,
				customURL:    msg.CustomURL,
			}
			if !m.ensureDeploymentLoaded(req.deploymentID, msg) {
				log.Printf("worker %d: skipping unknown deployment %s", workerID, req.deploymentID)
				continue
			}
			m.appendLog(req.deploymentID, "info", fmt.Sprintf("Worker %d picked up deployment", workerID))
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("worker %d: panic processing %s: %v", workerID, req.deploymentID, r)
						m.markFailed(req.deploymentID, fmt.Sprintf("internal build error (panic): %v", r))
					}
				}()
				m.buildAndRun(req.deploymentID, req.repoURL, req.displayRepo, req.branch, req.customURL)
			}()
		}
	}

	for req := range m.buildQueue {
		if !m.deploymentExists(req.deploymentID) {
			continue
		}
		m.appendLog(req.deploymentID, "info", fmt.Sprintf("Worker %d picked up deployment", workerID))
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("worker %d: panic processing %s: %v", workerID, req.deploymentID, r)
					m.markFailed(req.deploymentID, fmt.Sprintf("internal build error (panic): %v", r))
				}
			}()
			m.buildAndRun(req.deploymentID, req.repoURL, req.displayRepo, req.branch, req.customURL)
		}()
	}
}

func (m *Manager) deploymentExists(deploymentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			return true
		}
	}
	return false
}

func (m *Manager) markFailed(deploymentID, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			m.deployments[i].Status = "failed"
			m.deployments[i].Error = message
			if m.deployments[i].URL == "" {
				m.deployments[i].URL = "about:blank"
			}
			m.publishLocked(RuntimeEvent{
				Type:         "status",
				DeploymentID: deploymentID,
				UserID:       m.deployments[i].UserID,
				Status:       "failed",
				Timestamp:    time.Now().UTC(),
			})
			m.appendLogLocked(deploymentID, "error", message)
			m.persistDeploymentLocked(deploymentID)
			return
		}
	}
}

func (m *Manager) ensureDeploymentLoaded(deploymentID string, msg buildQueueMessage) bool {
	if m.deploymentExists(deploymentID) {
		return true
	}
	if m.store == nil {
		return false
	}
	dep, found, err := m.store.GetDeployment(deploymentID)
	if err != nil || !found {
		return false
	}
	m.mu.Lock()
	m.deployments = append(m.deployments, dep)
	if _, ok := m.logs[dep.ID]; !ok {
		m.logs[dep.ID] = []models.DeploymentLog{}
	}
	m.lastTouched[dep.ID] = time.Now().UTC()
	m.mu.Unlock()
	return true
}

func (m *Manager) updateStatus(deploymentID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			m.deployments[i].Status = status
			m.publishLocked(RuntimeEvent{
				Type:         "status",
				DeploymentID: deploymentID,
				UserID:       m.deployments[i].UserID,
				Status:       status,
				Timestamp:    time.Now().UTC(),
			})
			m.appendLogLocked(deploymentID, "info", fmt.Sprintf("Status changed to %s", status))
			m.persistDeploymentLocked(deploymentID)
			return
		}
	}
}

func (m *Manager) appendLog(deploymentID, level, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendLogLocked(deploymentID, level, message)
}

func (m *Manager) appendLogLocked(deploymentID, level, message string) {
	var ownerID string
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			ownerID = m.deployments[i].UserID
			break
		}
	}
	entry := models.DeploymentLog{Time: time.Now().UTC(), Level: level, Message: message}
	m.logs[deploymentID] = append(m.logs[deploymentID], entry)
	m.publishLocked(RuntimeEvent{
		Type:         "log",
		DeploymentID: deploymentID,
		UserID:       ownerID,
		Level:        level,
		Message:      message,
		Timestamp:    entry.Time,
	})
	go m.persistLog(deploymentID, entry)
}

func (m *Manager) persistDeploymentLocked(deploymentID string) {
	if m.store == nil {
		return
	}
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			snapshot := m.deployments[i]
			if err := m.store.UpsertDeployment(snapshot); err != nil {
				log.Printf("runtime persistence upsert failed for %s: %v", snapshot.ID, err)
			}
			return
		}
	}
}

func (m *Manager) persistLog(deploymentID string, entry models.DeploymentLog) {
	if m.store == nil {
		return
	}
	if err := m.store.AppendLog(deploymentID, entry); err != nil {
		log.Printf("runtime persistence log append failed for %s: %v", deploymentID, err)
	}
}

func (m *Manager) persistDelete(deploymentID string) {
	if m.store == nil {
		return
	}
	if err := m.store.DeleteDeployment(deploymentID); err != nil {
		log.Printf("runtime persistence delete failed for %s: %v", deploymentID, err)
	}
}

func (m *Manager) publishLocked(event RuntimeEvent) {
	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// ==================== BUILD HELPERS ====================

func (m *Manager) runDockerBuildWithRetries(tmpDir, image, dockerfilePath, deploymentID string) error {
	attempts := m.buildRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		ctxBuild, cancelBuild := context.WithTimeout(context.Background(), m.buildTimeout)
		err := runCmdDir(ctxBuild, tmpDir, "docker", dockerBuildArgs(image, dockerfilePath)...)
		cancelBuild()
		if err == nil {
			return nil
		}
		lastErr = err
		if !shouldRetryDockerError(err) {
			return err
		}
		if attempt < attempts {
			m.appendLog(deploymentID, "warn", fmt.Sprintf("Build attempt %d/%d failed, retrying", attempt, attempts))
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
}

func dockerBuildArgs(image, dockerfilePath string) []string {
	args := []string{"build"}
	if getBoolEnv("BUILD_PULL", true) {
		args = append(args, "--pull")
	}
	if getBoolEnv("BUILD_NO_CACHE", false) {
		args = append(args, "--no-cache")
	}
	if platform := strings.TrimSpace(os.Getenv("BUILD_PLATFORM")); platform != "" {
		args = append(args, "--platform", platform)
	}
	if target := strings.TrimSpace(os.Getenv("BUILD_TARGET")); target != "" {
		args = append(args, "--target", target)
	}
	return append(args, "-t", image, "-f", dockerfilePath, ".")
}

func checkDockerDaemonAvailable() error {
	dockerHost := strings.TrimSpace(os.Getenv("DOCKER_HOST"))

	// TCP connection (e.g. tcp://host.docker.internal:2375) — skip socket file check
	if strings.HasPrefix(dockerHost, "tcp://") || strings.HasPrefix(dockerHost, "http://") {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := runCmd(ctx, "docker", "info"); err != nil {
			if isDockerDaemonUnavailableError(err) {
				return fmt.Errorf("cannot connect to Docker daemon at %s — ensure Docker Desktop is running and 'Expose daemon on tcp://localhost:2375' is enabled in Docker Desktop settings", dockerHost)
			}
			return err
		}
		return nil
	}

	// Unix socket path
	socketPath := "/var/run/docker.sock"
	if dockerHost != "" {
		socketPath = strings.TrimPrefix(dockerHost, "unix://")
	}
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"Docker socket not found at %s — mount it in docker-compose: volumes: [/var/run/docker.sock:/var/run/docker.sock]",
			socketPath,
		)
	}

	// Validate we can connect to the unix socket. Opening socket files with
	// os.OpenFile can fail on some Docker Desktop setups even when Docker is
	// actually reachable, causing false negatives.
	conn, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		return fmt.Errorf(
			"Docker socket exists but is not accessible (permission denied) — add 'group_add: [\"999\"]' to the backend service in docker-compose.yml, or run the container as root: %v",
			err,
		)
	}
	_ = conn.Close()

	// Actually ping the daemon
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := runCmd(ctx, "docker", "info"); err != nil {
		if isDockerDaemonUnavailableError(err) {
			return errors.New("cannot connect to the Docker daemon; ensure Docker Desktop/daemon is running and reachable")
		}
		return err
	}
	return nil
}

func shouldRetryDockerError(err error) bool {
	if err == nil {
		return false
	}
	return !isDockerDaemonUnavailableError(err)
}

func isDockerDaemonUnavailableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "is the docker daemon running") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "error during connect") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "no such file or directory")
}

func dockerRunArgs(container, image, deploymentID string, hostPort, containerPort int) []string {
	args := []string{"run", "-d", "--name", container, "-p", fmt.Sprintf("%d:%d", hostPort, containerPort)}
	args = append(args,
		"--label", "instantdeploy.managed=true",
		"--label", fmt.Sprintf("instantdeploy.deployment_id=%s", deploymentID),
	)
	if memory := strings.TrimSpace(os.Getenv("RUN_MEMORY")); memory != "" {
		args = append(args, "--memory", memory)
	}
	if cpus := strings.TrimSpace(os.Getenv("RUN_CPUS")); cpus != "" {
		args = append(args, "--cpus", cpus)
	}
	if pids := strings.TrimSpace(os.Getenv("RUN_PIDS_LIMIT")); pids != "" {
		args = append(args, "--pids-limit", pids)
	}
	restart := strings.TrimSpace(os.Getenv("RUN_RESTART_POLICY"))
	if restart == "" {
		restart = "no"
	}
	return append(args, "--restart", restart, image)
}

func deployToKubernetes(name, image string, containerPort, nodePort int) error {
	yaml := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
	name: %s
	namespace: instantdeploy
	labels:
		app: %s
		instantdeploy.managed: "true"
spec:
	replicas: 1
	selector:
		matchLabels:
			app: %s
	template:
		metadata:
			labels:
				app: %s
				instantdeploy.managed: "true"
		spec:
			containers:
				- name: app
					image: %s
					imagePullPolicy: IfNotPresent
					ports:
						- containerPort: %d
					resources:
						requests:
							cpu: 100m
							memory: 128Mi
						limits:
							cpu: "1"
							memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
	name: %s
	namespace: instantdeploy
	labels:
		instantdeploy.managed: "true"
spec:
	type: NodePort
	selector:
		app: %s
	ports:
		- protocol: TCP
			port: %d
			targetPort: %d
			nodePort: %d
`, name, name, name, name, image, containerPort, name, name, containerPort, containerPort, nodePort)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := runCmdWithInput(ctx, yaml, "kubectl", "apply", "-f", "-"); err != nil {
		return err
	}
	ctxWait, cancelWait := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelWait()
	return runCmd(ctxWait, "kubectl", "rollout", "status", fmt.Sprintf("deployment/%s", name), "-n", "instantdeploy", "--timeout=60s")
}

func deleteKubernetesDeployment(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = runCmd(ctx, "kubectl", "delete", "service", name, "-n", "instantdeploy", "--ignore-not-found=true")
	_ = runCmd(ctx, "kubectl", "delete", "deployment", name, "-n", "instantdeploy", "--ignore-not-found=true")
}

func getExecutionModeFromEnv() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("EXECUTION_MODE")))
	if mode != "kubernetes" {
		return defaultExecutionMode
	}
	return "kubernetes"
}

func patchDeprecatedDockerBaseImage(dockerfilePath, buildErr string) (bool, string, error) {
	if !strings.Contains(strings.ToLower(buildErr), "not found") {
		return false, "", nil
	}
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read Dockerfile: %w", err)
	}
	content := string(data)

	replacements := []struct {
		pattern     *regexp.Regexp
		replacement string
		image       string
	}{
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:8-jdk-alpine`), `${1}eclipse-temurin:8-jdk`, "eclipse-temurin:8-jdk"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:8-jre-alpine`), `${1}eclipse-temurin:8-jre`, "eclipse-temurin:8-jre"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:11-jdk-alpine`), `${1}eclipse-temurin:11-jdk`, "eclipse-temurin:11-jdk"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:17-jdk-alpine`), `${1}eclipse-temurin:17-jdk`, "eclipse-temurin:17-jdk"},
	}
	for _, r := range replacements {
		if r.pattern.MatchString(content) {
			updated := r.pattern.ReplaceAllString(content, r.replacement)
			if updated == content {
				continue
			}
			if err := os.WriteFile(dockerfilePath, []byte(updated), 0o644); err != nil {
				return false, "", fmt.Errorf("failed to write patched Dockerfile: %w", err)
			}
			return true, r.image, nil
		}
	}
	return false, "", nil
}

func shouldUseJavaDockerFallback(repoDir, buildErr string) bool {
	errLower := strings.ToLower(buildErr)
	if !(strings.Contains(errLower, "build/dependency") && strings.Contains(errLower, "not found") && strings.Contains(errLower, "copy")) {
		return false
	}
	_, isJava, _ := javaBuildTool(repoDir)
	return isJava
}

func writeJavaDockerfile(repoDir string) (string, bool, error) {
	content, ok, err := javaBuildTool(repoDir)
	if err != nil || !ok {
		return "", false, err
	}
	df := filepath.Join(repoDir, "Dockerfile.instantdeploy.java")
	return df, true, os.WriteFile(df, []byte(content), 0o644)
}

func detectSimpleLanguage(repoDir string) string {
	// Priority: explicit project manifests first
	if fileExists(filepath.Join(repoDir, "package.json")) {
		return "node"
	}
	if fileExists(filepath.Join(repoDir, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(repoDir, "pom.xml")) || fileExists(filepath.Join(repoDir, "build.gradle")) || fileExists(filepath.Join(repoDir, "build.gradle.kts")) {
		return "java"
	}
	if fileExists(filepath.Join(repoDir, "Cargo.toml")) {
		return "rust"
	}
	if fileExists(filepath.Join(repoDir, "requirements.txt")) || fileExists(filepath.Join(repoDir, "pyproject.toml")) || fileExists(filepath.Join(repoDir, "Pipfile")) {
		return "python"
	}

	// Recursive marker scan for monorepos/example repos.
	lang := "unknown"
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".venv" || name == "venv" {
				return filepath.SkipDir
			}
			return nil
		}
		switch info.Name() {
		case "package.json":
			lang = "node"
			return filepath.SkipAll
		case "go.mod":
			lang = "go"
			return filepath.SkipAll
		case "pom.xml", "build.gradle", "build.gradle.kts":
			lang = "java"
			return filepath.SkipAll
		case "Cargo.toml":
			lang = "rust"
			return filepath.SkipAll
		case "requirements.txt", "pyproject.toml", "Pipfile":
			if lang == "unknown" {
				lang = "python"
			}
		}
		return nil
	})
	if lang != "unknown" {
		return lang
	}

	// Last resort: infer from Dockerfile base image.
	if fileExists(filepath.Join(repoDir, "Dockerfile")) {
		if b, err := os.ReadFile(filepath.Join(repoDir, "Dockerfile")); err == nil {
			content := strings.ToLower(string(b))
			switch {
			case strings.Contains(content, "from python"):
				return "python"
			case strings.Contains(content, "from node"):
				return "node"
			case strings.Contains(content, "from golang"):
				return "go"
			case strings.Contains(content, "from gradle") || strings.Contains(content, "from maven") || strings.Contains(content, "temurin"):
				return "java"
			case strings.Contains(content, "from rust"):
				return "rust"
			}
		}
	}

	return "unknown"
}

func writeSimpleDockerfile(repoDir, language string) (string, int, error) {
	path := filepath.Join(repoDir, "Dockerfile.instantdeploy.simple")
	var content string
	containerPort := 8080

	switch language {
	case "python":
		containerPort = 80
		content = `FROM python:3.11
WORKDIR /app
COPY . .
RUN ( [ -f requirements.txt ] && pip install -r requirements.txt || true ) && pip install gunicorn flask
EXPOSE 80
CMD ["sh", "-c", "python app.py || python main.py || gunicorn -b 0.0.0.0:80 app:app || gunicorn -b 0.0.0.0:80 main:app || gunicorn -b 0.0.0.0:80 httpbin:app || flask run --host=0.0.0.0 --port=80 || python -m http.server 80 --bind 0.0.0.0"]
`
	case "node":
		containerPort = 80
		content = `FROM node:18
WORKDIR /app
COPY . .
RUN npm install || npm install --legacy-peer-deps
RUN npm install -g serve
ENV PORT=80
EXPOSE 80
CMD ["sh", "-c", "npm start || node server.js || node index.js || serve -s . -l 80"]
`
	case "go":
		containerPort = 8080
		content = `FROM golang:1.21
WORKDIR /app
COPY . .
RUN go build -o app . || go build -o app ./cmd/server || go build -o app ./cmd/...
EXPOSE 8080
CMD ["./app", "-text=instantdeploy", "-listen=:8080"]
`
	case "java":
		containerPort = 8080
		content = `FROM gradle:7.6-jdk17
WORKDIR /app
COPY . .
RUN gradle build --no-daemon || gradle bootJar --no-daemon
EXPOSE 8080
CMD ["sh", "-c", "java -jar build/libs/*.jar"]
`
	case "rust":
		containerPort = 8080
		content = `FROM rust:1.75
WORKDIR /app
COPY . .
RUN cargo build --release
EXPOSE 8080
CMD ["sh", "-c", "bin=$(find target/release -maxdepth 1 -type f -perm -111 | grep -v '\\.d$' | head -n 1); if [ -n \"$bin\" ]; then \"$bin\"; else python -m http.server 8080 --bind 0.0.0.0; fi"]
`
	default:
		return "", 0, fmt.Errorf("unsupported language: %s", language)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", 0, err
	}

	return path, containerPort, nil
}

func javaBuildTool(repoDir string) (string, bool, error) {
	if fileExists(filepath.Join(repoDir, "pom.xml")) {
		return "FROM maven:3.9-eclipse-temurin-17 AS build\nWORKDIR /app\nCOPY pom.xml .\nCOPY src ./src\nRUN mvn -q -DskipTests package\nFROM eclipse-temurin:17-jre\nWORKDIR /app\nCOPY --from=build /app/target/*.jar /app/app.jar\nEXPOSE 8080\nCMD [\"java\",\"-jar\",\"/app/app.jar\"]\n", true, nil
	}
	if fileExists(filepath.Join(repoDir, "build.gradle")) || fileExists(filepath.Join(repoDir, "build.gradle.kts")) || fileExists(filepath.Join(repoDir, "gradlew")) {
		return "FROM gradle:8.7-jdk17 AS build\nWORKDIR /app\nCOPY . .\nRUN gradle bootJar --no-daemon || gradle build --no-daemon\nFROM eclipse-temurin:17-jre\nWORKDIR /app\nCOPY --from=build /app/build/libs/*.jar /app/app.jar\nEXPOSE 8080\nCMD [\"java\",\"-jar\",\"/app/app.jar\"]\n", true, nil
	}
	return "", false, nil
}

// ==================== GENERAL HELPERS ====================

func normalizeRepositoryInput(input string) (repoURL, displayRepo string, err error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", errors.New("repository is required")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", errors.New("invalid repository URL")
		}
		if !strings.EqualFold(u.Scheme, "https") && !strings.EqualFold(u.Scheme, "http") {
			return "", "", errors.New("repository URL must use http or https")
		}
		if !strings.EqualFold(u.Hostname(), "github.com") {
			return "", "", errors.New("repository URL must point to github.com")
		}
		parts := strings.Split(strings.Trim(strings.TrimSpace(u.Path), "/"), "/")
		if len(parts) < 2 {
			return "", "", errors.New("repository URL must include owner and repository")
		}
		owner := strings.TrimSpace(parts[0])
		repo := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".git")
		nameRE := regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
		if !nameRE.MatchString(owner) || !nameRE.MatchString(repo) {
			return "", "", errors.New("repository owner/name contains invalid characters")
		}
		displayRepo = owner + "/" + repo
		repoURL = fmt.Sprintf("https://github.com/%s.git", displayRepo)
		return repoURL, displayRepo, nil
	}
	if strings.Count(raw, "/") != 1 {
		return "", "", errors.New("repository must be owner/repo or a GitHub URL")
	}
	parts := strings.Split(raw, "/")
	nameRE := regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	if len(parts) != 2 || !nameRE.MatchString(parts[0]) || !nameRE.MatchString(parts[1]) {
		return "", "", errors.New("repository must be owner/repo using valid GitHub characters")
	}
	return fmt.Sprintf("https://github.com/%s.git", raw), raw, nil
}

// waitForContainerReady first verifies the Docker container reaches "running"
// state, then probes HTTP readiness through multiple network paths:
//   - localhost:<hostPort>       (works when backend runs on the host)
//   - host.docker.internal:<hostPort> (works from Docker Desktop containers)
//   - <containerIP>:<containerPort>   (works when backend is a sibling container)
func waitForContainerReady(container string, hostPort, containerPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	sawDirectoryListing := false

	// Phase 1: Wait for the container to reach "running" state (up to 15s)
	phase1Deadline := time.Now().Add(15 * time.Second)
	if phase1Deadline.After(deadline) {
		phase1Deadline = deadline
	}
	for time.Now().Before(phase1Deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", container)
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			status := strings.TrimSpace(string(out))
			switch status {
			case "running":
				goto httpCheck
			case "exited", "dead", "removing":
				return fmt.Errorf("container %s has status %q", container, status)
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("container %s did not start within 15s", container)

httpCheck:
	// Phase 2: Probe HTTP readiness through multiple network paths
	urls := []string{
		fmt.Sprintf("http://localhost:%d", hostPort),
		fmt.Sprintf("http://host.docker.internal:%d", hostPort),
	}

	// Also try the container's internal Docker IP (works for sibling containers)
	if containerIP := getContainerIP(container); containerIP != "" {
		urls = append(urls, fmt.Sprintf("http://%s:%d", containerIP, containerPort))
	}

	httpClient := &http.Client{Timeout: 3 * time.Second}

	for time.Now().Before(deadline) {
		// Re-check container hasn't crashed
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}", container)
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			status := strings.TrimSpace(string(out))
			if status == "exited" || status == "dead" {
				return fmt.Errorf("container %s crashed (status: %s)", container, status)
			}
		}

		for _, url := range urls {
			resp, err := httpClient.Get(url)
			if err == nil {
				bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				_ = resp.Body.Close()
				if resp.StatusCode < 500 {
					if isDirectoryListingResponse(resp.StatusCode, resp.Header.Get("Content-Type"), string(bodyBytes)) {
						sawDirectoryListing = true
						continue
					}
					return nil
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	if sawDirectoryListing {
		return fmt.Errorf("timeout after %s — app returned directory listing instead of an application response on ports %d/%d", timeout, hostPort, containerPort)
	}
	return fmt.Errorf("timeout after %s — container is running but app not reachable on ports %d/%d", timeout, hostPort, containerPort)
}

// getContainerIP returns the first IP address assigned to the container, or
// empty string if it cannot be determined.
func getContainerIP(container string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", container)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func waitForAppReady(hostPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	sawDirectoryListing := false
	urls := []string{
		fmt.Sprintf("http://localhost:%d", hostPort),
		fmt.Sprintf("http://host.docker.internal:%d", hostPort),
	}
	for time.Now().Before(deadline) {
		for _, url := range urls {
			resp, err := http.Get(url)
			if err == nil {
				bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				_ = resp.Body.Close()
				if resp.StatusCode < 500 {
					if isDirectoryListingResponse(resp.StatusCode, resp.Header.Get("Content-Type"), string(bodyBytes)) {
						sawDirectoryListing = true
						continue
					}
					return nil
				}
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
	if sawDirectoryListing {
		return fmt.Errorf("timeout after %s — app returned directory listing instead of an application response", timeout)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

func isDirectoryListingResponse(statusCode int, contentType, body string) bool {
	if statusCode >= 500 {
		return false
	}

	bodyLower := strings.ToLower(body)
	contentTypeLower := strings.ToLower(contentType)

	if strings.Contains(bodyLower, "directory listing for /") {
		return true
	}
	if strings.Contains(bodyLower, "<title>index of /") || strings.Contains(bodyLower, "<h1>index of /") {
		return true
	}
	if strings.Contains(contentTypeLower, "text/html") && strings.Contains(bodyLower, "parent directory") && strings.Contains(bodyLower, "href=") {
		return true
	}

	// Heuristic for plain-text/raw listings frequently seen when serving repo roots.
	// Example:
	//   .git/
	//   README.md
	//   setup.py
	//   requirements.txt
	lines := strings.Split(bodyLower, "\n")
	repoListingTokens := 0
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "/") {
			repoListingTokens++
			continue
		}
		if strings.HasSuffix(line, ".md") ||
			strings.HasSuffix(line, ".py") ||
			strings.HasSuffix(line, ".cfg") ||
			strings.HasSuffix(line, ".toml") ||
			strings.HasSuffix(line, ".json") ||
			strings.HasSuffix(line, ".yaml") ||
			strings.HasSuffix(line, ".yml") ||
			strings.HasSuffix(line, ".txt") ||
			line == "dockerfile" ||
			line == "requirements.txt" ||
			line == "setup.py" ||
			line == "package.json" ||
			line == "go.mod" {
			repoListingTokens++
		}
	}
	if repoListingTokens >= 4 && (strings.Contains(bodyLower, ".git/") || strings.Contains(bodyLower, "dockerfile")) {
		return true
	}

	return false
}

func getContainerLogs(container string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "80", container)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func inferPortFromContainerLogs(logs string) int {
	if strings.TrimSpace(logs) == "" {
		return 0
	}

	reCandidates := []*regexp.Regexp{
		regexp.MustCompile(`(?i)https?://[^\s:]+:(\d{2,5})`),
		regexp.MustCompile(`(?i)(?:running on|listening on|serving on)\s+(?:port\s+)?(\d{2,5})`),
		regexp.MustCompile(`(?i)port\s*[=:]\s*(\d{2,5})`),
	}

	counts := map[int]int{}
	for _, re := range reCandidates {
		for _, m := range re.FindAllStringSubmatch(logs, -1) {
			if len(m) < 2 {
				continue
			}
			p, err := strconv.Atoi(strings.TrimSpace(m[1]))
			if err != nil || p <= 0 || p > 65535 {
				continue
			}
			counts[p]++
		}
	}

	bestPort := 0
	bestCount := 0
	for p, c := range counts {
		if c > bestCount {
			bestPort = p
			bestCount = c
		}
	}

	return bestPort
}

func hasLoopbackBindHint(logs string) bool {
	if strings.TrimSpace(logs) == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)(running on|listening on|serving on).*(127\.0\.0\.1|localhost)`)
	return re.MatchString(logs)
}

func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnv(name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %v: %s", name, args, msg)
	}
	return nil
}

func runCmdOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnv(name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s %v: %s", name, args, msg)
	}
	return string(output), nil
}

func runCmdWithInput(ctx context.Context, input, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnv(name)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %v: %s", name, args, msg)
	}
	return nil
}

func runCmdDir(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = commandEnv(name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %v: %s", name, args, msg)
	}
	return nil
}

func commandEnv(name string) []string {
	_ = name
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
}

func findAvailablePort(min, max int) (int, error) {
	for i := 0; i < 40; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
		p := min + int(n.Int64())
		if isPortAvailable(p) {
			return p, nil
		}
	}
	return 0, errors.New("no available port found")
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func sanitizeName(v string) string {
	v = strings.ToLower(strings.ReplaceAll(v, "_", "-"))
	re := regexp.MustCompile(`[^a-z0-9-]`)
	v = re.ReplaceAllString(v, "")
	v = strings.Trim(v, "-")
	if v == "" {
		return "instantdeploy-runtime"
	}
	if len(v) > 50 {
		v = v[:50]
	}
	return v
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getBoolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func getBuildTimeoutFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("BUILD_TIMEOUT"))
	if v == "" {
		return defaultBuildTimeout
	}
	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		return defaultBuildTimeout
	}
	return time.Duration(seconds) * time.Second
}

func getBuildRetriesFromEnv() int {
	v := strings.TrimSpace(os.Getenv("MAX_BUILD_RETRIES"))
	if v == "" {
		return defaultBuildRetries
	}
	retries, err := strconv.Atoi(v)
	if err != nil || retries < 0 {
		return defaultBuildRetries
	}
	if retries > 5 {
		return 5
	}
	return retries
}

func getBuildWorkersFromEnv() int {
	v := strings.TrimSpace(os.Getenv("BUILD_WORKERS"))
	if v == "" {
		return defaultBuildWorkers
	}
	workers, err := strconv.Atoi(v)
	if err != nil || workers <= 0 {
		return defaultBuildWorkers
	}
	if workers > 10 {
		return 10
	}
	return workers
}

func getQueueKeyFromEnv() string {
	v := strings.TrimSpace(os.Getenv("BUILD_QUEUE_KEY"))
	if v == "" {
		return defaultQueueKey
	}
	return v
}
