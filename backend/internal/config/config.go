// Package config читает настройки сервера из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config — настройки приложения.
type Config struct {
	HTTPAddr    string        // адрес HTTP-сервера, например ":8080"
	DatabaseURL string        // строка подключения к PostgreSQL
	JWTSecret   string        // секрет подписи JWT (HS256)
	JWTTTL      time.Duration // срок жизни токена
	FrontendDir string        // каталог клиентской части (templates, static)
}

const minSecretLen = 32

// Load читает .env (если файл есть) и переменные окружения.
// Переменные окружения имеют приоритет над .env.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}

	cfg := Config{
		HTTPAddr:    getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		FrontendDir: getEnv("FRONTEND_DIR", "../frontend"),
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "30m"))
	if err != nil || ttl <= 0 {
		return Config{}, fmt.Errorf("JWT_TTL: некорректная длительность %q", os.Getenv("JWT_TTL"))
	}
	cfg.JWTTTL = ttl

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL не задана (см. backend/.env.example)")
	}
	if len(cfg.JWTSecret) < minSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET должна быть не короче %d символов", minSecretLen)
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
