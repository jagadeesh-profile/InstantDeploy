package api

import (
	"log"
	"net/http"
	"time"

	"instantdeploy/backend/internal/monitoring"
	"instantdeploy/backend/pkg/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(handler *Handler, metrics *monitoring.Metrics, corsOrigins []string, isDev bool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(utils.NewCORSMiddleware(corsOrigins, isDev))
	r.Use(metrics.HTTPMiddleware)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		utils.WriteJSON(w, http.StatusNotFound, map[string]any{
			"error":   "not_found",
			"path":    req.URL.Path,
			"message": "route not found — use /api/v1 for endpoint discovery",
		})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		log.Printf("405 method_not_allowed method=%s path=%s ua=%q remote=%s request_id=%s", req.Method, req.URL.Path, req.UserAgent(), req.RemoteAddr, middleware.GetReqID(req.Context()))
		utils.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error":  "method_not_allowed",
			"path":   req.URL.Path,
			"method": req.Method,
		})
	})

	r.Get("/ws", handler.WebSocket)
	r.Get("/", handler.APIRoot)
	r.Get("/api", handler.APIRoot)
	r.Get("/api/v1", handler.APIRoot)

	r.Group(func(rt chi.Router) {
		rt.Use(middleware.Timeout(60 * time.Second))

		// Public endpoints
		rt.Get("/health", handler.Health)
		rt.Get("/api/health", handler.Health)
		rt.Get("/api/v1/health", handler.Health)
		rt.Post("/api/v1/auth/signup", handler.SignUp)
		rt.Post("/api/v1/auth/verify", handler.VerifyAccount)
		rt.Post("/api/v1/auth/forgot-password", handler.ForgotPassword)
		rt.Post("/api/v1/auth/reset-password", handler.ResetPassword)
		rt.Post("/api/v1/auth/login", handler.Login)
		rt.Get("/metrics", promhttp.Handler().ServeHTTP)

		// Protected endpoints
		rt.Route("/api/v1", func(api chi.Router) {
			api.Get("/", handler.APIRoot)
			api.Group(func(private chi.Router) {
				private.Use(handler.AuthMiddleware)
				private.Get("/runtime/stats", handler.RuntimeStats)
				private.Get("/deployments", handler.ListDeployments)
				private.Post("/deployments", handler.CreateDeployment)
				private.Delete("/deployments/{id}", handler.DeleteDeployment)
				private.Get("/deployments/{id}/status", handler.GetDeploymentStatus)
				private.Get("/deployments/{id}/logs", handler.GetDeploymentLogs)
				private.Get("/repositories", handler.SearchRepositories)
			})
		})
	})

	return r
}

