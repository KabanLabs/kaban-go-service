package gateway

import (
	"encoding/json"
	"io"
	"net/http"

	"log/slog"

	"github.com/VACdotCS/kaban-go-service/internal/app/ws"
	"github.com/VACdotCS/kaban-go-service/internal/metrics"
)

type Event struct {
	WorkspaceID string      `json:"workspaceId"`
	UserId      string      `json:"userId"`
	Type        string      `json:"type"`
	Payload     interface{} `json:"payload"`
	Rev         int64       `json:"rev"`
}

type App struct {
	hub    *ws.Hub
	apiKey string
	log    *slog.Logger
}

func New(hub *ws.Hub, apiKey string, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	return &App{hub: hub, apiKey: apiKey, log: logger}
}

func (a *App) Run() error {
	http.HandleFunc("/event", a.handleEvent)
	a.log.Info("Gateway started")
	return nil
}

func (a *App) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("x-api-key")
	if authHeader == "" || authHeader != a.apiKey {
		a.log.Warn("Unauthorized request to /event")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.log.Warn("Read request body error", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		a.log.Warn("JSON unmarshal error", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	a.log.Info("Received event", "workspaceId", event.WorkspaceID, "type", event.Type)
	metrics.EventsReceived.WithLabelValues(event.Type).Inc()

	a.hub.BroadcastEvent(ws.BroadcastMessage{
		WorkspaceID: event.WorkspaceID,
		UserId:      event.UserId,
		Message:     body,
	})

	w.WriteHeader(http.StatusOK)
}
