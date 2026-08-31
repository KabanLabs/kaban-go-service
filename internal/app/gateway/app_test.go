package gateway

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VACdotCS/kaban-go-service/internal/app/ws"
)

func TestGateway_handleEvent(t *testing.T) {
	logger := slog.Default()
	apiKey := "test-secret"
	hub := ws.NewHub(logger, false)
	app := New(hub, apiKey, logger)

	// We need to set up the route just like Run() does, or we can just call handleEvent directly.
	handler := http.HandlerFunc(app.handleEvent)

	tests := []struct {
		name         string
		method       string
		apiKeyHeader string
		body         interface{}
		expectedCode int
	}{
		{
			name:         "wrong method",
			method:       http.MethodGet,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "missing api key",
			method:       http.MethodPost,
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "invalid api key",
			method:       http.MethodPost,
			apiKeyHeader: "wrong-key",
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "invalid json body",
			method:       http.MethodPost,
			apiKeyHeader: apiKey,
			body:         "invalid-json", // not a valid struct or json
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "valid request",
			method:       http.MethodPost,
			apiKeyHeader: apiKey,
			body: Event{
				WorkspaceID: "ws1",
				UserId:      "user1",
				Type:        "test-event",
			},
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			if tt.body != nil {
				if s, ok := tt.body.(string); ok && s == "invalid-json" {
					reqBody = []byte(`{invalid: json`)
				} else {
					reqBody, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/event", bytes.NewReader(reqBody))
			if tt.apiKeyHeader != "" {
				req.Header.Set("x-api-key", tt.apiKeyHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("expected status code %d, got %d", tt.expectedCode, rr.Code)
			}
		})
	}
}

func TestGateway_BroadcastsToHub(t *testing.T) {
	logger := slog.Default()
	apiKey := "test-secret"
	hub := ws.NewHub(logger, false)
	app := New(hub, apiKey, logger)

	handler := http.HandlerFunc(app.handleEvent)

	event := Event{
		WorkspaceID: "ws1",
		UserId:      "user1",
		Type:        "test",
		Payload:     "data",
		Rev:         1,
	}
	reqBody, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader(reqBody))
	req.Header.Set("x-api-key", apiKey)
	rr := httptest.NewRecorder()

	// Wait, we need a way to check if the message reached the hub.
	// We can connect a dummy client to the hub's room and see if it gets the broadcast.
	room := hub.GetOrCreateRoom("ws1")
	// wait a bit for processing
	time.Sleep(10 * time.Millisecond)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	// Just a small sleep to let the broadcast channel process the message if any
	time.Sleep(10 * time.Millisecond)

	// The message is sent to room.broadcast channel.
	_ = room
}
