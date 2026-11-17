package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"значения по умолчанию", map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": secret}, ""},
		{"нет DATABASE_URL", map[string]string{"JWT_SECRET": secret}, "DATABASE_URL"},
		{"короткий секрет", map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "short"}, "JWT_SECRET"},
		{"неверный JWT_TTL", map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": secret, "JWT_TTL": "полчаса"}, "JWT_TTL"},
		{"отрицательный JWT_TTL", map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": secret, "JWT_TTL": "-5m"}, "JWT_TTL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"HTTP_ADDR", "DATABASE_URL", "JWT_SECRET", "JWT_TTL"} {
				t.Setenv(k, tt.env[k])
			}
			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ожидалась ошибка про %s, получено %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.HTTPAddr != ":8080" || cfg.JWTTTL != 30*time.Minute || cfg.DatabaseURL != "postgres://x" {
				t.Errorf("cfg = %+v", cfg)
			}
		})
	}

	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("JWT_TTL", "15m")
	cfg, err := Load()
	if err != nil || cfg.HTTPAddr != "127.0.0.1:9000" || cfg.JWTTTL != 15*time.Minute {
		t.Errorf("переменные окружения: %+v, %v", cfg, err)
	}
}
