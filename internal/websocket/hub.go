package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// Client WebSocket 客户端
type Client struct {
	hub  *Hub
	conn *ws.Conn
	send chan []byte
}

// Hub WebSocket 消息中心
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	stop       chan struct{}
	stopOnce   sync.Once
}

// NewHub 创建 WebSocket Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
	}
}

// Run 运行 Hub 消息循环
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[WebSocket] 客户端已连接，当前连接数: %d", count)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[WebSocket] 客户端已断开，当前连接数: %d", count)

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 发送缓冲区满，断开客户端
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// 广播通道满，丢弃消息
	}
}

// Stop shuts down the hub loop and closes all client send channels.
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)

		h.mu.Lock()
		defer h.mu.Unlock()
		for client := range h.clients {
			delete(h.clients, client)
			if client.conn != nil {
				client.conn.Close()
			}
			close(client.send)
		}
	})
}

// HandleWebSocket 处理 WebSocket 连接的 Gin handler
// tokenValidator: 可选，用于验证 ?token= query param，传 nil 则不验证（已在外层 auth 中间件保护时使用）
func (h *Hub) HandleWebSocket(tokenValidator ...func(token string) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果提供了 validator，验证 token query param
		if len(tokenValidator) > 0 && tokenValidator[0] != nil {
			token := c.Query("token")
			if token == "" || !tokenValidator[0](token) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[WebSocket] 升级失败: %v", err)
			return
		}

		client := &Client{
			hub:  h,
			conn: conn,
			send: make(chan []byte, 256),
		}

		select {
		case h.register <- client:
		case <-h.stop:
			close(client.send)
			conn.Close()
			return
		}

		// 启动读写协程
		go client.writePump()
		go client.readPump()
	}
}

// readPump 读取客户端消息（主要用于检测断开）
func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
		select {
		case c.hub.unregister <- c:
		case <-c.hub.stop:
		}
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump 向客户端发送消息
func (c *Client) writePump() {
	defer c.conn.Close()

	for msg := range c.send {
		if err := c.conn.WriteMessage(ws.TextMessage, msg); err != nil {
			break
		}
	}
}

// ClientCount 获取当前连接数
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
