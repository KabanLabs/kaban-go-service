package app

import (
	"log/slog"

	"github.com/VACdotCS/kaban-go-service/internal/app/gateway"
	"github.com/VACdotCS/kaban-go-service/internal/app/ws"
	"github.com/VACdotCS/kaban-go-service/internal/config"
)

// App объединяет все слои
type App struct {
	WS      *ws.App
	Gateway *gateway.App
	Logger  *slog.Logger
	Config  *config.Config
}

// New создаёт новый App со всеми слоями
func New(cfg *config.Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}

	wsApp := ws.New(&cfg.Ws, logger)
	gwApp := gateway.New(wsApp.Hub, logger)

	return &App{
		WS:      wsApp,
		Gateway: gwApp,
		Logger:  logger,
		Config:  cfg,
	}
}

// Run запускает все слои приложения
func (a *App) Run() error {
	if err := a.WS.Run(); err != nil {
		return err
	}

	if err := a.Gateway.Run(); err != nil {
		return err
	}

	a.Logger.Info("All application layers started")
	return nil
}
