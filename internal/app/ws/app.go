package ws

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/VACdotCS/kaban-go-service/internal/config"
	"github.com/VACdotCS/kaban-go-service/internal/services/auth"
	"github.com/gorilla/websocket"
)

type App struct {
	Hub    *Hub
	Config *config.WsConfig
	auth   *auth.Client
	log    *slog.Logger
}

type Client struct {
	conn          *websocket.Conn
	send          chan []byte
	userId        string
	logDataStream bool
}

type Hub struct {
	clients       map[string]map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	Broadcast     chan BroadcastMessage
	mu            sync.Mutex
	log           *slog.Logger
	logDataStream bool
}

type BroadcastMessage struct {
	WorkspaceID string
	UserId      string
	Message     []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewHub(logger *slog.Logger, logDataStream bool) *Hub {
	return &Hub{
		clients:       make(map[string]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		Broadcast:     make(chan BroadcastMessage),
		log:           logger,
		logDataStream: logDataStream,
	}
}

func New(cfg *config.WsConfig, authClient *auth.Client, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}

	hub := NewHub(logger, cfg.LogDataStream)
	return &App{
		Hub:    hub,
		Config: cfg,
		auth:   authClient,
		log:    logger,
	}
}

func (a *App) Run() error {
	// Регистрируем endpoint WS
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspaceId")
		userId := r.URL.Query().Get("userId")
		token := r.URL.Query().Get("token")

		if workspaceID == "" {
			http.Error(w, "workspaceId required", http.StatusBadRequest)
			return
		}
		if userId == "" {
			http.Error(w, "userId required", http.StatusBadRequest)
			return
		}
		if token == "" {
			http.Error(w, "token required", http.StatusBadRequest)
			return
		}

		isValid, err := a.auth.ValidateToken(r.Context(), token)
		if err != nil || !isValid {
			a.log.Warn("WebSocket unauthorized access attempt", "workspaceId", workspaceID, "userId", userId, "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		a.ServeWS(w, r, workspaceID, userId)
	})

	// Запускаем Hub
	go a.Hub.Run()

	a.log.Info("WebSocket server started")
	return nil
}

func (a *App) ServeWS(w http.ResponseWriter, r *http.Request, workspaceID, userId string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.Warn("WebSocket upgrade error", "error", err)
		return
	}

	client := &Client{
		conn:          conn,
		send:          make(chan []byte, a.Config.WSSendBuffer),
		userId:        userId,
		logDataStream: a.Config.LogDataStream,
	}

	key := fmt.Sprintf("%s", workspaceID)

	a.Hub.mu.Lock()
	if a.Hub.clients[key] == nil {
		a.Hub.clients[key] = make(map[*Client]bool)
	}
	a.Hub.clients[key][client] = true
	a.Hub.mu.Unlock()

	a.log.Info("New WS client connected", "workspaceId", workspaceID, "userId", userId, "addr", conn.RemoteAddr().String())

	go client.writePump(a.log)
	go client.readPump(a.Hub, workspaceID, userId, a.log)
}

func (c *Client) readPump(h *Hub, workspaceID, userId string, logger *slog.Logger) {
	defer func() {
		key := fmt.Sprintf("%s", workspaceID)
		h.mu.Lock()
		delete(h.clients[key], c)
		h.mu.Unlock()
		c.conn.Close()
		logger.Info("WS client disconnected", "workspaceId", workspaceID, "userId", userId, "addr", c.conn.RemoteAddr().String())
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			logger.Warn("ReadMessage error", "error", err, "workspaceId", workspaceID, "userId", userId)
			break
		}

		if c.logDataStream {
			logger.Info("WS message received from client", "workspaceId", workspaceID, "userId", userId, "messageSize", len(message))
		}

		h.Broadcast <- BroadcastMessage{
			WorkspaceID: workspaceID,
			UserId:      userId,
			Message:     message,
		}
	}
}

func (c *Client) writePump(logger *slog.Logger) {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			logger.Warn("WriteMessage error", "error", err, "userId", c.userId)
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
			if h.logDataStream {
				h.log.Info("WS client registered", "userId", client.userId)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			for _, clients := range h.clients {
				delete(clients, client)
			}
			h.mu.Unlock()
			close(client.send)
			if h.logDataStream {
				h.log.Info("WS client unregistered", "userId", client.userId)
			}

		case msg := <-h.Broadcast:
			if h.logDataStream {
				h.log.Info("WS broadcast received", "workspaceId", msg.WorkspaceID, "fromUserId", msg.UserId, "messageSize", len(msg.Message))
			}

			h.mu.Lock()
			clients := h.clients[msg.WorkspaceID]
			for client := range clients {
				if client.userId == msg.UserId {
					continue
				}

				if h.logDataStream {
					h.log.Info("WS sending event to client", "workspaceId", msg.WorkspaceID, "fromUserId", msg.UserId, "toUserId", client.userId)
				}

				select {
				case client.send <- msg.Message:
				default:
					if h.logDataStream {
						h.log.Warn("WS client send buffer full, dropping client", "workspaceId", msg.WorkspaceID, "toUserId", client.userId)
					}
					close(client.send)
					delete(clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}
