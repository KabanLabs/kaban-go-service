package gateway

import (
	"encoding/json"
	"io/ioutil"
	_ "log"
	"net/http"

	"github.com/VACdotCS/kaban-go-service/internal/app/ws"
)

type Event struct {
	WorkspaceID string      `json:"workspaceId"`
	Type        string      `json:"type"`
	Payload     interface{} `json:"payload"`
	Rev         int64       `json:"rev"`
}

func RunGateway(hub *ws.Hub) {
	http.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var event Event
		if err := json.Unmarshal(body, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Отправляем в hub
		hub.Broadcast <- ws.BroadcastMessage{
			WorkspaceID: event.WorkspaceID,
			Message:     body,
		}

		w.WriteHeader(http.StatusOK)
	})
}
