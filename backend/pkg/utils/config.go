package utils

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port             string
	JWTSecret        string
	JWTExpiryMinutes int
	GitHubToken      string
	DatabaseURL      string
	RedisAddr        string
	CORSOrigins      []string // Allowed CORS origins; empty = same-origin only
	Env              string   // "development" or "production"
}

// IsDev returns true when ENV is "development" or unset.
func (c Config) IsDev() bool {
	return c.Env == "" || strings.EqualFold(c.Env, "development")
}

func LoadConfig() Config {
	env := getEnv("ENV", "development")
	jwtSecret := os.Getenv("JWT_SECRET")

	if jwtSecret == "" {
		if strings.EqualFold(env, "production") {
			log.Fatal("FATAL: JWT_SECRET environment variable is required in production")
		}
		// Generate a random secret for development — unique per process restart
		jwtSecret = generateRandomSecret(32)
		log.Printf("WARNING: JWT_SECRET not set — generated ephemeral secret for development (tokens will not survive restarts)")
	}

	if len(jwtSecret) < 16 {
		log.Printf("WARNING: JWT_SECRET is shorter than 16 characters — this is insecure")
	}

	var corsOrigins []string
	if raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS")); raw != "" {
		for _, origin := range strings.Split(raw, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				corsOrigins = append(corsOrigins, origin)
			}
		}
	}

	return Config{
		Port:             getEnv("PORT", "8080"),
		JWTSecret:        jwtSecret,
		JWTExpiryMinutes: getEnvInt("JWT_EXPIRY_MINUTES", 120),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		CORSOrigins:      corsOrigins,
		Env:              env,
	}
}

func generateRandomSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("FATAL: failed to generate random JWT secret: %v", err)
	}
	return hex.EncodeToString(bytes)
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

