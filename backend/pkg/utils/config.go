package utils

import (
	"os"
	"strconv"
)

type Config struct {
	Port             string
	JWTSecret        string
	JWTExpiryMinutes int
	GitHubToken      string
	DatabaseURL      string
	RedisAddr        string
}

func LoadConfig() Config {
	return Config{
		Port:             getEnv("PORT", "8080"),
		JWTSecret:        getEnv("JWT_SECRET", "instantdeploy-dev-secret"),
		JWTExpiryMinutes: getEnvInt("JWT_EXPIRY_MINUTES", 120),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	out, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return out
}
