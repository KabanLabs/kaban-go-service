package ws

import (
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

type Room struct {
	id            string
	clients       map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	broadcast     chan BroadcastMessage
	log           *slog.Logger
	logDataStream bool
}

type Hub struct {
	rooms         map[string]*Room
	mu            sync.RWMutex
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

func NewRoom(id string, logger *slog.Logger, logDataStream bool) *Room {
	return &Room{
		id:            id,
		clients:       make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan BroadcastMessage),
		log:           logger,
		logDataStream: logDataStream,
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.register:
			r.clients[client] = true

			if r.logDataStream {
				r.log.Info("WS client registered to room", "workspaceId", r.id, "userId", client.userId)
			}

		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)

				if r.logDataStream {
					r.log.Info("WS client unregistered from room", "workspaceId", r.id, "userId", client.userId)
				}
			}
		case msg := <-r.broadcast:
			if r.logDataStream {
				r.log.Info("WS broadcast received in room", "workspaceId", r.id, "fromUserId", msg.UserId, "messageSize", len(msg.Message))
			}

			for client := range r.clients {
				if client.userId == msg.UserId {
					continue
				}

				if r.logDataStream {
					r.log.Info("WS sending event to client", "workspaceId", r.id, "fromUserId", msg.UserId, "toUserId", client.userId)
				}

				select {
				case client.send <- msg.Message:
				default:
					if r.logDataStream {
						r.log.Warn("WS client send buffer full, dropping client", "workspaceId", r.id, "toUserId", client.userId)
					}
					close(client.send)
					delete(r.clients, client)
				}
			}
		}
	}
}

func NewHub(logger *slog.Logger, logDataStream bool) *Hub {
	return &Hub{
		rooms:         make(map[string]*Room),
		log:           logger,
		logDataStream: logDataStream,
	}
}

func (h *Hub) GetOrCreateRoom(workspaceID string) *Room {
	h.mu.RLock()
	room, exists := h.rooms[workspaceID]
	h.mu.RUnlock()

	if !exists {
		h.mu.Lock()
		// Double-checked locking
		room, exists = h.rooms[workspaceID]
		if !exists {
			room = NewRoom(workspaceID, h.log, h.logDataStream)
			h.rooms[workspaceID] = room
			go room.Run()
		}
		h.mu.Unlock()
	}

	return room
}

func (h *Hub) BroadcastEvent(msg BroadcastMessage) {
	room := h.GetOrCreateRoom(msg.WorkspaceID)
	room.broadcast <- msg
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
			a.log.Warn("WebSocket connection attempt without workspaceId")
			http.Error(w, "workspaceId required", http.StatusBadRequest)
			return
		}
		if userId == "" {
			a.log.Warn("WebSocket connection attempt without userId")
			http.Error(w, "userId required", http.StatusBadRequest)
			return
		}
		if token == "" {
			a.log.Warn("WebSocket connection attempt without token", "workspaceId", workspaceID, "userId", userId)
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

	room := a.Hub.GetOrCreateRoom(workspaceID)
	room.register <- client

	a.log.Info("New WS client connected", "workspaceId", workspaceID, "userId", userId, "addr", conn.RemoteAddr().String())

	go client.writePump(a.log)
	go client.readPump(room, workspaceID, userId, a.log)
}

func (c *Client) readPump(r *Room, workspaceID, userId string, logger *slog.Logger) {
	defer func() {
		r.unregister <- c
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

		r.broadcast <- BroadcastMessage{
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
