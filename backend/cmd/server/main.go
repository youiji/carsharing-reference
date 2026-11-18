// Команда server запускает сервис каршеринга: REST API (/api/...), страницы интерфейса
// и фоновую отмену просроченных бронирований.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carsharing/internal/app"
	"carsharing/internal/config"
	"carsharing/internal/repository"
	"carsharing/internal/util"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Контекст отменяется по Ctrl+C или SIGTERM — запускается плавная остановка.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router, err := app.NewHandler(pool, util.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL), cfg.FrontendDir)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Порт занимается до сообщения о запуске: «server started» в логе означает,
	// что сервер действительно принимает соединения.
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("адрес %s уже занят: возможно, сервер уже запущен в другом терминале "+
				"(найти процесс: lsof -i tcp:%s, остановить: kill <PID>)", cfg.HTTPAddr, port(cfg.HTTPAddr))
		}
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	slog.Info("server started", "addr", cfg.HTTPAddr, "url", "http://localhost:"+port(cfg.HTTPAddr))

	// Просроченные бронирования снимаются каждые 15 секунд.
	go app.RunBookingExpiry(ctx, pool, 15*time.Second)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	slog.Info("server stopped")
	return nil
}

// port возвращает номер порта из адреса вида ":8080" или "127.0.0.1:8080".
func port(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
		return p
	}
	return addr
}
