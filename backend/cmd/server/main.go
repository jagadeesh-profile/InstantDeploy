package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"instantdeploy/backend/internal/api"
	"instantdeploy/backend/internal/auth"
	"instantdeploy/backend/internal/database"
	"instantdeploy/backend/internal/monitoring"
	"instantdeploy/backend/internal/repository"
	"instantdeploy/backend/internal/runtime"
	"instantdeploy/backend/internal/websocket"
	"instantdeploy/backend/pkg/utils"
)

func main() {
	cfg := utils.LoadConfig()
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTExpiryMinutes)

	ctx := context.Background()

	pgPool, dbErr := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if dbErr != nil {
		log.Printf("database unavailable, falling back to in-memory: %v", dbErr)
	}
	if pgPool != nil {
		defer pgPool.Close()
	}

	redisClient, redisErr := database.NewRedisClient(ctx, cfg.RedisAddr)
	if redisErr != nil {
		log.Printf("redis unavailable, falling back to in-memory build queue: %v", redisErr)
	}
	if redisClient != nil {
		defer redisClient.Close()
		log.Printf("durable build queue enabled (redis)")
	} else {
		log.Printf("durable build queue disabled (in-memory fallback)")
	}

	var store runtime.Store
	var userStore *database.UserStore
	if pgPool != nil {
		store = database.NewDeploymentStore(pgPool)
		userStore = database.NewUserStore(pgPool)
		if userStore != nil {
			if err := userStore.EnsureSchema(); err != nil {
				log.Printf("user schema setup failed, falling back to in-memory auth: %v", err)
				userStore = nil
			}
		}
		log.Printf("runtime persistence enabled (postgres)")
	} else {
		log.Printf("runtime persistence disabled (in-memory fallback)")
	}

	runtimeManager := runtime.NewManagerWithStoreAndQueue(store, redisClient)
	repoClient := repository.NewGitHubClient(cfg.GitHubToken)
	metrics := monitoring.NewMetrics()
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Forward runtime events to WebSocket hub
	subID, events := runtimeManager.Subscribe(512)
	_ = subID
	go func() {
		for event := range events {
			switch event.Type {
			case "status":
				wsHub.BroadcastDeploymentUpdate(event.UserID, event.DeploymentID, event.Status, map[string]any{
					"timestamp": event.Timestamp,
				})
			case "log":
				wsHub.BroadcastLog(event.UserID, event.DeploymentID, event.Level, event.Message)
			}
		}
	}()

	handler := api.NewHandler(jwtManager, runtimeManager, repoClient, metrics, wsHub, userStore, cfg.IsDev())
	router := api.NewRouter(handler, metrics, cfg.CORSOrigins, cfg.IsDev())

	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownErrCh := make(chan error, 1)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("shutdown signal received: %s", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		shutdownErrCh <- server.Shutdown(ctx)
	}()

	log.Printf("InstantDeploy backend listening on %s (env=%s)", addr, cfg.Env)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
	if err := <-shutdownErrCh; err != nil {
		log.Printf("graceful shutdown completed with error: %v", err)
	} else {
		log.Printf("graceful shutdown completed")
	}
}
