package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[string]map[*Client]bool // workspaceID -> clients
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

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.conn.RemoteAddr().String()] == nil {
				h.clients[client.conn.RemoteAddr().String()] = make(map[*Client]bool)
			}
			h.clients[client.conn.RemoteAddr().String()][client] = true
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

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, workspaceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.register <- client

	go client.writePump()
	go client.readPump(h, workspaceID)
}

func (c *Client) readPump(h *Hub, workspaceID string) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		h.Broadcast <- BroadcastMessage{
			WorkspaceID: workspaceID,
			Message:     message,
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		err := c.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			break
		}
	}
}
