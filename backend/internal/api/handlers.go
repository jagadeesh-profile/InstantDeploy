package api

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"instantdeploy/backend/internal/auth"
	"instantdeploy/backend/internal/database"
	"instantdeploy/backend/internal/monitoring"
	"instantdeploy/backend/internal/repository"
	"instantdeploy/backend/internal/runtime"
	"instantdeploy/backend/internal/websocket"
	"instantdeploy/backend/pkg/models"
	"instantdeploy/backend/pkg/utils"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// Handler wires all HTTP handlers together.
type Handler struct {
	jwt     *auth.JWTManager
	runtime *runtime.Manager
	repos   *repository.GitHubClient
	metrics *monitoring.Metrics
	wsHub   *websocket.Hub
	store   *database.UserStore
	usersMu sync.RWMutex
	users   map[string]storedUser
}

type storedUser struct {
	Username         string
	Email            string
	PasswordHash     []byte
	Role             string
	Verified         bool
	VerificationCode string
	FailedAttempts   int
	LockedUntil      time.Time
}

func NewHandler(
	jwt *auth.JWTManager,
	rt *runtime.Manager,
	repos *repository.GitHubClient,
	metrics *monitoring.Metrics,
	wsHub *websocket.Hub,
	store *database.UserStore,
) *Handler {
	// Demo password: Demo123! — passes all validation rules
	demoHash := mustHashPassword("Demo123!")

	h := &Handler{
		jwt:     jwt,
		runtime: rt,
		repos:   repos,
		metrics: metrics,
		wsHub:   wsHub,
		store:   store,
		users: map[string]storedUser{
			"demo": {
				Username:     "demo",
				Email:        "demo@instantdeploy.local",
				PasswordHash: demoHash,
				Role:         "developer",
				Verified:     true,
			},
		},
	}

	if h.store != nil {
		_, found, err := h.store.GetByUsername("demo")
		if err == nil && !found {
			_ = h.store.CreateUser(database.UserRecord{
				Username:     "demo",
				Email:        "demo@instantdeploy.local",
				PasswordHash: demoHash,
				Role:         "developer",
				Verified:     true,
			})
		}
	}
	return h
}

// ==================== USER HELPERS ====================

func userFromRecord(rec database.UserRecord) storedUser {
	return storedUser{
		Username:         rec.Username,
		Email:            rec.Email,
		PasswordHash:     rec.PasswordHash,
		Role:             rec.Role,
		Verified:         rec.Verified,
		VerificationCode: rec.VerificationCode,
		FailedAttempts:   rec.FailedAttempts,
		LockedUntil:      rec.LockedUntil,
	}
}

func userToRecord(user storedUser) database.UserRecord {
	return database.UserRecord{
		Username:         user.Username,
		Email:            user.Email,
		PasswordHash:     user.PasswordHash,
		Role:             user.Role,
		Verified:         user.Verified,
		VerificationCode: user.VerificationCode,
		FailedAttempts:   user.FailedAttempts,
		LockedUntil:      user.LockedUntil,
	}
}

func (h *Handler) getUserByUsername(username string) (storedUser, bool, error) {
	if h.store != nil {
		rec, found, err := h.store.GetByUsername(username)
		if err != nil {
			return storedUser{}, false, err
		}
		if !found {
			return storedUser{}, false, nil
		}
		return userFromRecord(rec), true, nil
	}
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()
	user, exists := h.users[username]
	return user, exists, nil
}

func (h *Handler) getUserByUsernameOrEmail(username, email string) (storedUser, bool, error) {
	if h.store != nil {
		rec, found, err := h.store.GetByUsernameOrEmail(username, email)
		if err != nil {
			return storedUser{}, false, err
		}
		if !found {
			return storedUser{}, false, nil
		}
		return userFromRecord(rec), true, nil
	}
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()
	if username != "" {
		if user, exists := h.users[username]; exists {
			return user, true, nil
		}
	}
	if email != "" {
		for _, user := range h.users {
			if strings.EqualFold(user.Email, email) {
				return user, true, nil
			}
		}
	}
	return storedUser{}, false, nil
}

func (h *Handler) emailExists(email string) (bool, error) {
	if h.store != nil {
		return h.store.EmailExists(email)
	}
	h.usersMu.RLock()
	defer h.usersMu.RUnlock()
	for _, user := range h.users {
		if strings.EqualFold(user.Email, email) {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) createUser(user storedUser) error {
	if h.store != nil {
		return h.store.CreateUser(userToRecord(user))
	}
	h.usersMu.Lock()
	defer h.usersMu.Unlock()
	h.users[user.Username] = user
	return nil
}

func (h *Handler) updateUser(user storedUser) error {
	if h.store != nil {
		return h.store.UpdateUser(userToRecord(user))
	}
	h.usersMu.Lock()
	defer h.usersMu.Unlock()
	h.users[user.Username] = user
	return nil
}

// ==================== SYSTEM ENDPOINTS ====================

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	runtimeStats, err := h.runtime.Stats()
	if err != nil {
		utils.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "degraded",
			"time":   time.Now().UTC(),
			"runtime": map[string]any{
				"queueMode": h.runtime.QueueMode(),
				"error":     err.Error(),
			},
		})
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
		"runtime": map[string]any{
			"queueMode":   runtimeStats.QueueMode,
			"queueDepth":  runtimeStats.QueueDepth,
			"workers":     runtimeStats.Workers,
			"deployments": runtimeStats.Deployments,
			"logs":        runtimeStats.Logs,
		},
	})
}

func (h *Handler) APIRoot(w http.ResponseWriter, _ *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"service": "instantdeploy-backend",
		"status":  "ok",
		"api":     "v1",
		"endpoints": map[string]string{
			"health":             "/api/v1/health",
			"auth_signup":        "/api/v1/auth/signup",
			"auth_verify":        "/api/v1/auth/verify",
			"auth_forgot":        "/api/v1/auth/forgot-password",
			"auth_reset":         "/api/v1/auth/reset-password",
			"auth_login":         "/api/v1/auth/login",
			"runtime_stats":      "/api/v1/runtime/stats",
			"deployments_list":   "/api/v1/deployments",
			"deployments_create": "/api/v1/deployments",
			"metrics":            "/metrics",
			"websocket":          "/ws",
		},
	})
}

func (h *Handler) RuntimeStats(w http.ResponseWriter, _ *http.Request) {
	stats, err := h.runtime.Stats()
	if err != nil {
		utils.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, stats)
}

// ==================== AUTH REQUEST TYPES ====================

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type signUpRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyRequest struct {
	Username string `json:"username"`
	Code     string `json:"code"`
}

type forgotPasswordRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type resetPasswordRequest struct {
	Username    string `json:"username"`
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

// ==================== AUTH HANDLERS ====================

func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	if username == "" || email == "" || password == "" {
		utils.WriteError(w, http.StatusBadRequest, "username, email and password are required")
		return
	}
	if err := validateUsername(username); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEmail(email); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validatePassword(password); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, exists, err := h.getUserByUsername(username); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to load user")
		return
	} else if exists {
		utils.WriteError(w, http.StatusConflict, "username already exists")
		return
	}

	if emailInUse, err := h.emailExists(email); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to check email")
		return
	} else if emailInUse {
		utils.WriteError(w, http.StatusConflict, "email already registered")
		return
	}

	hashed, err := hashPassword(password)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	verificationCode := newVerificationCode()
	if err := h.createUser(storedUser{
		Username:         username,
		Email:            email,
		PasswordHash:     hashed,
		Role:             "developer",
		Verified:         false,
		VerificationCode: verificationCode,
	}); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, map[string]any{
		"message":           "account created — verify before logging in",
		"verification_code": verificationCode,
		"user": models.User{
			ID:       username,
			Username: username,
			Role:     "developer",
		},
	})
}

func (h *Handler) VerifyAccount(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	username := strings.TrimSpace(req.Username)
	code := strings.TrimSpace(req.Code)
	if username == "" || code == "" {
		utils.WriteError(w, http.StatusBadRequest, "username and code are required")
		return
	}

	user, exists, err := h.getUserByUsername(username)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if !exists {
		utils.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.Verified {
		utils.WriteJSON(w, http.StatusOK, map[string]any{"message": "account already verified"})
		return
	}
	if user.VerificationCode != code {
		utils.WriteError(w, http.StatusBadRequest, "invalid verification code")
		return
	}

	user.Verified = true
	user.VerificationCode = ""
	if err := h.updateUser(user); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"message": "account verified"})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	if username == "" && email == "" {
		utils.WriteError(w, http.StatusBadRequest, "username or email is required")
		return
	}

	user, found, err := h.getUserByUsernameOrEmail(username, email)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if !found {
		utils.WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	code := newVerificationCode()
	user.VerificationCode = code
	if err := h.updateUser(user); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"message":    "password reset code generated",
		"reset_code": code,
	})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	username := strings.TrimSpace(req.Username)
	code := strings.TrimSpace(req.Code)
	newPassword := strings.TrimSpace(req.NewPassword)
	if username == "" || code == "" || newPassword == "" {
		utils.WriteError(w, http.StatusBadRequest, "username, code, and newPassword are required")
		return
	}
	if err := validatePassword(newPassword); err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, exists, err := h.getUserByUsername(username)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if !exists {
		utils.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.VerificationCode == "" || user.VerificationCode != code {
		utils.WriteError(w, http.StatusBadRequest, "invalid reset code")
		return
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to process password")
		return
	}
	user.PasswordHash = hash
	user.VerificationCode = ""
	user.FailedAttempts = 0
	user.LockedUntil = time.Time{}
	if err := h.updateUser(user); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"message": "password reset successful"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		utils.WriteError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, exists, err := h.getUserByUsername(username)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if !exists {
		utils.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if !user.LockedUntil.IsZero() && time.Now().Before(user.LockedUntil) {
		utils.WriteError(w, http.StatusTooManyRequests, "account temporarily locked due to repeated failed logins")
		return
	}
	if !checkPassword(user.PasswordHash, password) {
		user.FailedAttempts++
		if user.FailedAttempts >= 5 {
			user.LockedUntil = time.Now().Add(15 * time.Minute)
			user.FailedAttempts = 0
		}
		_ = h.updateUser(user)
		utils.WriteError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if !user.Verified {
		utils.WriteError(w, http.StatusUnauthorized, "account not verified")
		return
	}

	user.FailedAttempts = 0
	user.LockedUntil = time.Time{}
	_ = h.updateUser(user)

	token, err := h.jwt.Generate(username)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": models.User{
			ID:       user.Username,
			Username: user.Username,
			Role:     user.Role,
		},
	})
}

// ==================== MIDDLEWARE ====================

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.WriteError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		claims, err := h.jwt.Validate(token)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		r.Header.Set("X-User", claims.Subject)
		next.ServeHTTP(w, r)
	})
}

// ==================== DEPLOYMENT HANDLERS ====================

func (h *Handler) ListDeployments(w http.ResponseWriter, _ *http.Request) {
	deployments := h.runtime.List()
	utils.WriteJSON(w, http.StatusOK, map[string]any{"items": deployments})
}

func (h *Handler) GetDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		utils.WriteError(w, http.StatusBadRequest, "deployment id is required")
		return
	}
	logs, err := h.runtime.Logs(id)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"items": logs})
}

func (h *Handler) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		utils.WriteError(w, http.StatusBadRequest, "deployment id is required")
		return
	}
	if err := h.runtime.Delete(id); err != nil {
		utils.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"message": "deployment deleted"})
}

type createDeploymentRequest struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	URL        string `json:"url"`
}

func (h *Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req createDeploymentRequest
	if err := utils.DecodeJSON(r, &req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Repository) == "" {
		utils.WriteError(w, http.StatusBadRequest, "repository is required (owner/repo or GitHub URL)")
		return
	}
	if strings.TrimSpace(req.Branch) == "" {
		req.Branch = "main"
	}

	customURL := strings.TrimSpace(req.URL)
	if customURL != "" {
		parsed, err := url.ParseRequestURI(customURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			utils.WriteError(w, http.StatusBadRequest, "url must be a valid absolute URL")
			return
		}
	}

	requestedBy := strings.TrimSpace(r.Header.Get("X-User"))
	created, err := h.runtime.Create(req.Repository, req.Branch, customURL, requestedBy)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.metrics.DeploymentsCreated.Inc()
	utils.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) SearchRepositories(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		utils.WriteError(w, http.StatusBadRequest, "query is required")
		return
	}
	repos, err := h.repos.Search(r.Context(), query)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, "failed to fetch repositories")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"items": repos})
}

// ==================== WEBSOCKET ====================

func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	if h.wsHub == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "websocket hub unavailable")
		return
	}
	websocket.ServeWS(h.wsHub, w, r)
}

// ==================== VALIDATION & CRYPTO ====================

func validateUsername(username string) error {
	re := regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)
	if !re.MatchString(username) {
		return fmt.Errorf("username must be 3-30 chars: letters, numbers, underscores only")
	}
	return nil
}

func validateEmail(email string) error {
	re := regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	if !re.MatchString(email) {
		return fmt.Errorf("email is invalid")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()-_=+[]{}:;,.?", r):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password must include uppercase, lowercase, number, and special character")
	}
	return nil
}

func newVerificationCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func hashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func checkPassword(hash []byte, password string) bool {
	if len(hash) == 0 {
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
}

func mustHashPassword(password string) []byte {
	hash, err := hashPassword(password)
	if err != nil {
		panic(err)
	}
	return hash
}
