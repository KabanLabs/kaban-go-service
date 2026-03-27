package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VACdotCS/kaban-go-service/internal/app"
	"github.com/VACdotCS/kaban-go-service/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	envLocal = "local"
	envDev   = "envDev"
	envProd  = "prod"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.MustLoad()

	// Настраиваем логгер
	log := setupLogger(cfg.Env)
	log.Info("Starting application")

	// Создаём приложение
	myApp := app.New(cfg, log)

	// Запускаем все слои приложения
	if err := myApp.Run(); err != nil {
		log.Error("Failed to start application layers", "error", err)
		os.Exit(1)
	}

	http.Handle("/metrics", promhttp.Handler())

	// HTTP сервер
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Ws.Port),
		Handler: http.DefaultServeMux,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Ws.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("Server failed", "error", err)
		}
	}()

	// Обработка сигналов завершения
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(nil, 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown failed", "error", err)
	} else {
		log.Info("Server stopped gracefully")
	}

	fmt.Println("Bye!")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
