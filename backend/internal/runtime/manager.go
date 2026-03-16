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
)

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
	for i := 0; i < workerCount; i++ {
		go m.workerLoop(i + 1)
	}
	go m.cleanupInactiveLoop()
	return m
}

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
	deployments, depErr := m.store.ListDeployments()
	if depErr != nil {
		log.Printf("runtime persistence load deployments failed: %v", depErr)
		return
	}
	logsByDeployment, logsErr := m.store.ListLogsByDeployment()
	if logsErr != nil {
		log.Printf("runtime persistence load logs failed: %v", logsErr)
		return
	}

	m.deployments = deployments
	m.logs = logsByDeployment
	now := time.Now().UTC()
	for i := range deployments {
		id := deployments[i].ID
		if _, ok := m.logs[id]; !ok {
			m.logs[id] = []models.DeploymentLog{}
		}
		m.lastTouched[id] = now

		// Deployments stuck mid-execution when the server last stopped cannot
		// resume; mark them failed so the UI doesn't show them as stuck.
		// Exception: in Redis-queue mode "queued" jobs are still in the Redis
		// queue and will be picked up by the next worker — don't fail those.
		status := deployments[i].Status
		usingRedis := m.redisQueue != nil
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
		deploymentID: deployment.ID,
		repoURL:      repoURL,
		displayRepo:  displayRepo,
		branch:       branch,
		customURL:    customURL,
	}) {
		m.markFailed(deployment.ID, "build queue is full, try again in a few moments")
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
		_ = runCmd(context.Background(), "docker", "rm", "-f", dep.Container)
	}
	if dep.Image != "" {
		_ = runCmd(context.Background(), "docker", "rmi", "-f", dep.Image)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// re-find in case concurrent mutation happened
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

func (m *Manager) cleanupInactiveLoop() {
	ticker := time.NewTicker(idleCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupInactiveDeployments()
	}
}

func (m *Manager) cleanupInactiveDeployments() {
	cutoff := time.Now().UTC().Add(-idleDeploymentTTL)
	staleIDs := make([]string, 0)

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
}

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

	dockerfilePath, containerPort, err := ensureDockerfile(tmpDir, func(level, message string) {
		m.appendLog(deploymentID, level, message)
	})
	if err != nil {
		m.markFailed(deploymentID, err.Error())
		return
	}
	m.appendLog(deploymentID, "info", fmt.Sprintf("Using Dockerfile: %s", filepath.Base(dockerfilePath)))

	image := sanitizeName("instantdeploy-" + deploymentID)
	container := sanitizeName("instantdeploy-" + deploymentID)

	hostPort, err := findAvailablePort(20000, 39999)
	if err != nil {
		m.markFailed(deploymentID, fmt.Sprintf("failed to allocate host port: %v", err))
		return
	}
	m.appendLog(deploymentID, "info", fmt.Sprintf("Allocated host port %d", hostPort))

	m.updateStatus(deploymentID, "building")
	m.appendLog(deploymentID, "info", "Checking Docker daemon availability")
	if dockerErr := checkDockerDaemonAvailable(); dockerErr != nil {
		m.markFailed(deploymentID, fmt.Sprintf("docker unavailable: %v", dockerErr))
		return
	}
	m.appendLog(deploymentID, "info", "Building Docker image")
	buildErr := m.runDockerBuildWithRetries(tmpDir, image, dockerfilePath, deploymentID)
	if buildErr != nil {
		patched, replacement, patchErr := patchDeprecatedDockerBaseImage(dockerfilePath, buildErr.Error())
		if patchErr != nil {
			m.markFailed(deploymentID, fmt.Sprintf("docker build failed: %v", buildErr))
			return
		}
		if patched {
			m.appendLog(deploymentID, "warn", fmt.Sprintf("Detected deprecated base image; patched Dockerfile to use %s", replacement))
			m.appendLog(deploymentID, "info", "Retrying Docker image build")
			buildErr = m.runDockerBuildWithRetries(tmpDir, image, dockerfilePath, deploymentID)
		}

		if buildErr != nil && shouldUseJavaDockerFallback(tmpDir, buildErr.Error()) {
			fallbackDockerfile, ok, fallbackErr := writeJavaDockerfile(tmpDir)
			if fallbackErr != nil {
				m.markFailed(deploymentID, fmt.Sprintf("docker build failed: %v", buildErr))
				return
			}
			if ok {
				dockerfilePath = fallbackDockerfile
				containerPort = 8080
				m.appendLog(deploymentID, "warn", "Repository Dockerfile requires prebuilt artifacts; using Java fallback Dockerfile")
				m.appendLog(deploymentID, "info", "Retrying Docker image build with Java fallback")
				buildErr = m.runDockerBuildWithRetries(tmpDir, image, dockerfilePath, deploymentID)
			}
		}

		if buildErr != nil {
			m.markFailed(deploymentID, fmt.Sprintf("docker build failed: %v", buildErr))
			return
		}
	}
	m.appendLog(deploymentID, "info", "Docker image built")

	_ = runCmd(context.Background(), "docker", "rm", "-f", container)

	m.updateStatus(deploymentID, "starting")
	m.appendLog(deploymentID, "info", "Starting container")
	ctxRun, cancelRun := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelRun()
	runArgs := dockerRunArgs(container, image, hostPort, containerPort)
	if err := runCmd(ctxRun, "docker", runArgs...); err != nil {
		m.markFailed(deploymentID, fmt.Sprintf("docker run failed: %v", err))
		return
	}

	m.appendLog(deploymentID, "info", "Waiting for application readiness")
	if readyErr := waitForAppReady(hostPort, 45*time.Second); readyErr != nil {
		containerLogs := getContainerLogs(container)
		if containerLogs != "" {
			m.appendLog(deploymentID, "error", containerLogs)
		}
		m.markFailed(deploymentID, fmt.Sprintf("app failed to become reachable: %v", readyErr))
		return
	}

	localURL := fmt.Sprintf("http://localhost:%d", hostPort)
	finalURL := strings.TrimSpace(customURL)
	if finalURL == "" {
		finalURL = localURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			m.deployments[i].Status = "running"
			m.publishLocked(RuntimeEvent{
				Type:         "status",
				DeploymentID: deploymentID,
				UserID:       m.deployments[i].UserID,
				Status:       "running",
				Timestamp:    time.Now().UTC(),
			})
			m.deployments[i].URL = finalURL
			m.deployments[i].LocalURL = localURL
			m.deployments[i].Image = image
			m.deployments[i].Container = container
			m.deployments[i].Repository = displayRepo
			m.persistDeploymentLocked(deploymentID)
			m.appendLogLocked(deploymentID, "info", fmt.Sprintf("Deployment is live at %s", localURL))
			return
		}
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
		if pushErr := m.redisQueue.LPush(ctx, m.queueKey, payload).Err(); pushErr != nil {
			return false
		}
		return true
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
			if unmarshalErr := json.Unmarshal([]byte(result[1]), &msg); unmarshalErr != nil {
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
			m.appendLog(req.deploymentID, "info", fmt.Sprintf("Worker %d picked queued deployment", workerID))
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("worker %d: recovered from panic processing %s: %v", workerID, req.deploymentID, r)
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
		m.appendLog(req.deploymentID, "info", fmt.Sprintf("Worker %d picked queued deployment", workerID))
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("worker %d: recovered from panic processing %s: %v", workerID, req.deploymentID, r)
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
			m.publishLocked(RuntimeEvent{
				Type:         "status",
				DeploymentID: deploymentID,
				UserID:       m.deployments[i].UserID,
				Status:       "failed",
				Timestamp:    time.Now().UTC(),
			})
			m.appendLogLocked(deploymentID, "error", message)
			if m.deployments[i].URL == "" {
				m.deployments[i].URL = "about:blank"
			}
			m.persistDeploymentLocked(deploymentID)
			return
		}
	}
}

// ensureDeploymentLoaded checks if deploymentID exists in memory. If not and a
// persistent store is available, it tries to load it from there (so that Redis
// workers on any backend instance can process jobs created by other instances).
// Returns true if the deployment is now in memory and ready to be processed.
func (m *Manager) ensureDeploymentLoaded(deploymentID string, msg buildQueueMessage) bool {
	if m.deploymentExists(deploymentID) {
		return true
	}
	if m.store == nil {
		return false
	}
	dep, found, err := m.store.GetDeployment(deploymentID)
	if err != nil {
		log.Printf("ensureDeploymentLoaded: store lookup failed for %s: %v", deploymentID, err)
		return false
	}
	if !found {
		return false
	}
	// Load into local memory so the worker can operate on it.
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
	logEntry := models.DeploymentLog{
		Time:    time.Now().UTC(),
		Level:   level,
		Message: message,
	}
	m.logs[deploymentID] = append(m.logs[deploymentID], logEntry)
	m.publishLocked(RuntimeEvent{
		Type:         "log",
		DeploymentID: deploymentID,
		UserID:       ownerID,
		Level:        level,
		Message:      message,
		Timestamp:    logEntry.Time,
	})
	go m.persistLog(deploymentID, logEntry)
}

func (m *Manager) persistDeploymentLocked(deploymentID string) {
	if m.store == nil {
		return
	}
	for i := range m.deployments {
		if m.deployments[i].ID == deploymentID {
			snapshot := m.deployments[i]
			go func() {
				if err := m.store.UpsertDeployment(snapshot); err != nil {
					log.Printf("runtime persistence upsert failed for %s: %v", snapshot.ID, err)
				}
			}()
			return
		}
	}
}

func (m *Manager) persistLog(deploymentID string, logEntry models.DeploymentLog) {
	if m.store == nil {
		return
	}
	if err := m.store.AppendLog(deploymentID, logEntry); err != nil {
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

func normalizeRepositoryInput(input string) (repoURL, displayRepo string, err error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", errors.New("repository is required")
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if !strings.Contains(raw, "github.com/") {
			return "", "", errors.New("repository URL must be a GitHub URL")
		}
		repoURL = strings.TrimSuffix(raw, "/")
		if !strings.HasSuffix(repoURL, ".git") {
			repoURL += ".git"
		}
		displayRepo = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(raw, "https://github.com/"), "http://github.com/"), ".git")
		return repoURL, displayRepo, nil
	}

	if strings.Count(raw, "/") != 1 {
		return "", "", errors.New("repository must be owner/repo or a GitHub URL")
	}

	repoURL = fmt.Sprintf("https://github.com/%s.git", raw)
	return repoURL, raw, nil
}

func ensureDockerfile(repoDir string, logf RuntimeLogger) (dockerfilePath string, containerPort int, err error) {
	detector := NewSmartDetector()
	project, err := detector.Detect(repoDir)
	if err != nil {
		return "", 0, err
	}
	if logf != nil && project != nil {
		logf("info", fmt.Sprintf("Detected project: %s", project.Summary))
	}

	fixer := NewBuildFixer(logf)
	if err := fixer.Fix(repoDir, project); err != nil {
		return "", 0, err
	}

	generator := NewDockerfileGenerator(logf)
	return generator.Generate(repoDir, project)
}

func detectExposePortFromDockerfile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`(?im)^\s*EXPOSE\s+(\d+)`)
	m := re.FindStringSubmatch(string(data))
	if len(m) < 2 {
		return 0
	}
	var port int
	_, _ = fmt.Sscanf(m[1], "%d", &port)
	return port
}

func waitForAppReady(hostPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	urls := []string{
		fmt.Sprintf("http://localhost:%d", hostPort),
		fmt.Sprintf("http://host.docker.internal:%d", hostPort),
	}

	for time.Now().Before(deadline) {
		for _, url := range urls {
			resp, err := http.Get(url)
			if err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode < 500 {
					return nil
				}
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}

	return fmt.Errorf("timeout after %s", timeout)
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

func patchDeprecatedDockerBaseImage(dockerfilePath, buildErr string) (bool, string, error) {
	errLower := strings.ToLower(buildErr)
	if !strings.Contains(errLower, "not found") {
		return false, "", nil
	}

	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read Dockerfile for patching: %w", err)
	}
	content := string(data)

	replacements := []struct {
		pattern     *regexp.Regexp
		replacement string
		image       string
	}{
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:8-jdk-alpine(\s+AS\s+\S+)?\s*$`), `${1}eclipse-temurin:8-jdk${2}`, "eclipse-temurin:8-jdk"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:8-jre-alpine(\s+AS\s+\S+)?\s*$`), `${1}eclipse-temurin:8-jre${2}`, "eclipse-temurin:8-jre"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:11-jdk-alpine(\s+AS\s+\S+)?\s*$`), `${1}eclipse-temurin:11-jdk${2}`, "eclipse-temurin:11-jdk"},
		{regexp.MustCompile(`(?im)^(\s*FROM\s+)openjdk:17-jdk-alpine(\s+AS\s+\S+)?\s*$`), `${1}eclipse-temurin:17-jdk${2}`, "eclipse-temurin:17-jdk"},
	}

	for _, r := range replacements {
		if r.pattern.MatchString(content) {
			updated := r.pattern.ReplaceAllString(content, r.replacement)
			if updated == content {
				continue
			}
			if writeErr := os.WriteFile(dockerfilePath, []byte(updated), 0o644); writeErr != nil {
				return false, "", fmt.Errorf("failed to write patched Dockerfile: %w", writeErr)
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
	builderContent, ok, err := javaBuildTool(repoDir)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}

	df := filepath.Join(repoDir, "Dockerfile.instantdeploy.java")
	if writeErr := os.WriteFile(df, []byte(builderContent), 0o644); writeErr != nil {
		return "", false, fmt.Errorf("failed to create generated Java Dockerfile: %w", writeErr)
	}
	return df, true, nil
}

func javaBuildTool(repoDir string) (string, bool, error) {
	if fileExists(filepath.Join(repoDir, "pom.xml")) {
		content := "FROM maven:3.9-eclipse-temurin-17 AS build\nWORKDIR /app\nCOPY pom.xml .\nCOPY src ./src\nRUN mvn -q -DskipTests package\nFROM eclipse-temurin:17-jre\nWORKDIR /app\nCOPY --from=build /app/target/*.jar /app/app.jar\nEXPOSE 8080\nCMD [\"java\",\"-jar\",\"/app/app.jar\"]\n"
		return content, true, nil
	}
	if fileExists(filepath.Join(repoDir, "build.gradle")) || fileExists(filepath.Join(repoDir, "build.gradle.kts")) || fileExists(filepath.Join(repoDir, "gradlew")) {
		content := "FROM gradle:8.7-jdk17 AS build\nWORKDIR /app\nCOPY . .\nRUN gradle bootJar --no-daemon || gradle build --no-daemon\nFROM eclipse-temurin:17-jre\nWORKDIR /app\nCOPY --from=build /app/build/libs/*.jar /app/app.jar\nEXPOSE 8080\nCMD [\"java\",\"-jar\",\"/app/app.jar\"]\n"
		return content, true, nil
	}
	return "", false, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
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
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
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

func findAvailablePort(min, max int) (int, error) {
	if min < 1024 || max <= min {
		return 0, errors.New("invalid port range")
	}

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
	v = strings.ToLower(v)
	v = strings.ReplaceAll(v, "_", "-")
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

func (m *Manager) runDockerBuildWithRetries(tmpDir, image, dockerfilePath, deploymentID string) error {
	attempts := m.buildRetries + 1
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		ctxBuild, cancelBuild := context.WithTimeout(context.Background(), m.buildTimeout)
		buildArgs := dockerBuildArgs(image, dockerfilePath)
		err := runCmdDir(ctxBuild, tmpDir, "docker", buildArgs...)
		cancelBuild()
		if err == nil {
			return nil
		}

		lastErr = err
		if !shouldRetryDockerError(err) {
			m.appendLog(deploymentID, "error", "Docker build failed with a non-retryable runtime error")
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
	args = append(args, "-t", image, "-f", dockerfilePath, ".")
	return args
}

func checkDockerDaemonAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := runCmd(ctx, "docker", "info"); err != nil {
		if isDockerDaemonUnavailableError(err) {
			return errors.New("cannot connect to the Docker daemon; ensure Docker Desktop/daemon is running and reachable")
		}
		if isDockerBuildxUnavailableError(err) {
			return errors.New("docker buildx is not available; install/enable Docker Buildx")
		}
		return err
	}
	return nil
}

func shouldRetryDockerError(err error) bool {
	if err == nil {
		return false
	}
	if isDockerDaemonUnavailableError(err) || isDockerBuildxUnavailableError(err) {
		return false
	}
	return true
}

func isDockerDaemonUnavailableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cannot connect to the docker daemon") ||
		strings.Contains(msg, "is the docker daemon running") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "error during connect")
}

func isDockerBuildxUnavailableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "install the buildx component") ||
		strings.Contains(msg, "docker buildx")
}

func dockerRunArgs(container, image string, hostPort, containerPort int) []string {
	args := []string{"run", "-d", "--name", container, "-p", fmt.Sprintf("%d:%d", hostPort, containerPort)}

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
		restart = "unless-stopped"
	}
	args = append(args, "--restart", restart, image)
	return args
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
	default:
		return fallback
	}
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
