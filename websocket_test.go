package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()

	// Start a test server with the WebSocket handler.
	server := httptest.NewServer(HandleWebSocket(hub))
	defer server.Close()

	// Connect a client.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}

	// Give the goroutine a moment to register.
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	conn.Close()
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after close, got %d", hub.ClientCount())
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()

	server := httptest.NewServer(HandleWebSocket(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Connect two clients.
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	// Broadcast a message.
	hub.Broadcast("reload")

	// Both clients should receive it.
	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read error: %v", i, err)
		}
		if string(msg) != "reload" {
			t.Errorf("client %d got %q, want %q", i, string(msg), "reload")
		}
	}
}

func TestHub_BroadcastRemovesDeadClient(t *testing.T) {
	hub := NewHub()

	server := httptest.NewServer(HandleWebSocket(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	// Connect two clients.
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}

	// Close one client. The read loop goroutine will call Unregister.
	conn1.Close()
	time.Sleep(100 * time.Millisecond)

	// Broadcast — the dead client should be cleaned up.
	hub.Broadcast("reload")
	time.Sleep(50 * time.Millisecond)

	// The surviving client should still receive messages.
	conn2.SetReadDeadline(time.Now().Add(time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("surviving client read error: %v", err)
	}
	if string(msg) != "reload" {
		t.Errorf("got %q, want %q", string(msg), "reload")
	}
}

func TestHub_CloseAll(t *testing.T) {
	hub := NewHub()

	server := httptest.NewServer(HandleWebSocket(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	hub.CloseAll()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after CloseAll, got %d", hub.ClientCount())
	}
}
