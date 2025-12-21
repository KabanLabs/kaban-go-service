package app

import (
	"log"
	"net/http"

	"github.com/VACdotCS/kaban-go-service/internal/app/gateway"
	"github.com/VACdotCS/kaban-go-service/internal/app/ws"
)

func App() {
	hub := ws.NewHub()
	go hub.Run()

	// Запускаем WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspaceId")
		if workspaceID == "" {
			http.Error(w, "workspaceId required", http.StatusBadRequest)
			return
		}
		hub.ServeWS(w, r, workspaceID)
	})

	// Запускаем HTTP POST gateway
	gateway.RunGateway(hub)

	log.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
