package main

import (
	"context"
	"log"
	"net/http"

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

	handler := api.NewHandler(jwtManager, runtimeManager, repoClient, metrics, wsHub, userStore)
	router := api.NewRouter(handler, metrics)

	addr := ":" + cfg.Port
	log.Printf("InstantDeploy backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
