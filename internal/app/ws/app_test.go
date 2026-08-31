package ws

import (
	"log/slog"
	"testing"
	"time"
)

func TestHub_GetOrCreateRoom(t *testing.T) {
	logger := slog.Default()
	hub := NewHub(logger, false)

	room1 := hub.GetOrCreateRoom("ws1")
	if room1 == nil {
		t.Fatal("expected room, got nil")
	}
	if room1.id != "ws1" {
		t.Errorf("expected room id ws1, got %s", room1.id)
	}

	room2 := hub.GetOrCreateRoom("ws1")
	if room1 != room2 {
		t.Error("expected same room instance for same id")
	}

	room3 := hub.GetOrCreateRoom("ws2")
	if room1 == room3 {
		t.Error("expected different room instances for different ids")
	}
}

func TestRoom_BroadcastEvent(t *testing.T) {
	logger := slog.Default()
	hub := NewHub(logger, false)
	room := hub.GetOrCreateRoom("ws1")

	client1 := &Client{
		send:   make(chan []byte, 10),
		userId: "user1",
	}
	client2 := &Client{
		send:   make(chan []byte, 10),
		userId: "user2",
	}

	room.register <- client1
	room.register <- client2

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	msg := BroadcastMessage{
		WorkspaceID: "ws1",
		UserId:      "user1",
		Message:     []byte("test message"),
	}

	hub.BroadcastEvent(msg)

	// Wait for broadcast
	time.Sleep(10 * time.Millisecond)

	// user1 shouldn't receive its own message
	select {
	case <-client1.send:
		t.Error("client1 should not receive its own message")
	default:
	}

	// user2 should receive the message
	select {
	case receivedMsg := <-client2.send:
		if string(receivedMsg) != "test message" {
			t.Errorf("expected 'test message', got %s", string(receivedMsg))
		}
	default:
		t.Error("client2 should have received the message")
	}

	room.unregister <- client1
	room.unregister <- client2
}
