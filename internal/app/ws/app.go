package ws

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/VACdotCS/kaban-go-service/internal/config"
	"github.com/gorilla/websocket"
)

type App struct {
	Hub    *Hub
	Config *config.WsConfig
	log    *slog.Logger
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	Broadcast  chan BroadcastMessage
	mu         sync.Mutex
}

type BroadcastMessage struct {
	WorkspaceID string
	Message     []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		Broadcast:  make(chan BroadcastMessage),
	}
}

func New(cfg *config.WsConfig, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}

	hub := NewHub()
	return &App{
		Hub:    hub,
		Config: cfg,
		log:    logger,
	}
}

func (a *App) Run() error {
	// Регистрируем endpoint WS
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspaceId")
		if workspaceID == "" {
			http.Error(w, "workspaceId required", http.StatusBadRequest)
			return
		}
		a.ServeWS(w, r, workspaceID)
	})

	// Запускаем Hub
	go a.Hub.Run()

	a.log.Info("WebSocket server started")
	return nil
}

func (a *App) ServeWS(w http.ResponseWriter, r *http.Request, workspaceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.Warn("WebSocket upgrade error", "error", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, a.Config.WSSendBuffer),
	}

	a.Hub.mu.Lock()
	if a.Hub.clients[workspaceID] == nil {
		a.Hub.clients[workspaceID] = make(map[*Client]bool)
	}
	a.Hub.clients[workspaceID][client] = true
	a.Hub.mu.Unlock()

	a.log.Info("New WS client connected", "workspaceId", workspaceID, "addr", conn.RemoteAddr().String())

	go client.writePump(a.log)
	go client.readPump(a.Hub, workspaceID, a.log)
}

func (c *Client) readPump(h *Hub, workspaceID string, logger *slog.Logger) {
	defer func() {
		h.mu.Lock()
		delete(h.clients[workspaceID], c)
		h.mu.Unlock()
		c.conn.Close()
		logger.Info("WS client disconnected", "workspaceId", workspaceID, "addr", c.conn.RemoteAddr().String())
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			logger.Warn("ReadMessage error", "error", err)
			break
		}
		h.Broadcast <- BroadcastMessage{
			WorkspaceID: workspaceID,
			Message:     message,
		}
	}
}

func (c *Client) writePump(logger *slog.Logger) {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			logger.Warn("WriteMessage error", "error", err)
			break
		}
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Вставка клиента во все workspace (или можно конкретизировать)
			for _, clients := range h.clients {
				clients[client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			for _, clients := range h.clients {
				delete(clients, client)
			}
			h.mu.Unlock()
			close(client.send)

		case msg := <-h.Broadcast:
			h.mu.Lock()
			clients := h.clients[msg.WorkspaceID]
			for client := range clients {
				select {
				case client.send <- msg.Message:
				default:
					close(client.send)
					delete(clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}
