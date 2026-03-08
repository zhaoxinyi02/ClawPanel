package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func waitForCondition(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestNewHubInitialState(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}
	if hub.ClientCount() != 0 {
		t.Fatalf("ClientCount() = %d, want 0", hub.ClientCount())
	}
	if cap(hub.broadcast) != 256 {
		t.Fatalf("broadcast buffer = %d, want 256", cap(hub.broadcast))
	}
}

func TestHubRunRegistersBroadcastsAndUnregisters(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	hub.register <- client
	waitForCondition(t, func() bool { return hub.ClientCount() == 1 })

	hub.Broadcast([]byte("hello"))
	select {
	case msg := <-client.send:
		if string(msg) != "hello" {
			t.Fatalf("broadcast message = %q, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}

	hub.unregister <- client
	waitForCondition(t, func() bool { return hub.ClientCount() == 0 })
}

func TestHubRunDropsSlowClientWhenBufferFull(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}
	client.send <- []byte("busy")

	hub.register <- client
	waitForCondition(t, func() bool { return hub.ClientCount() == 1 })

	hub.Broadcast([]byte("new-message"))
	waitForCondition(t, func() bool { return hub.ClientCount() == 0 })
}

func TestHandleWebSocketRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := NewHub()
	router := gin.New()
	router.GET("/ws", hub.HandleWebSocket(func(token string) bool {
		return token == "ok"
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws?token=bad", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
