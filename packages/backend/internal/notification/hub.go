package notification

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait             = 10 * time.Second
	pongWait              = 60 * time.Second
	pingPeriod            = 30 * time.Second
	maxSendBacklog        = 16
	defaultBatchSize      = 200
	defaultBroadcastDelay = 10 * time.Millisecond
)

type Hub struct {
	mu         sync.Mutex
	clients    map[*hubClient]struct{}
	closed     bool
	batchSize  int
	batchDelay time.Duration
	onBatch    func(size int)
}

type hubClient struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*hubClient]struct{}),
		batchSize:  defaultBatchSize,
		batchDelay: defaultBroadcastDelay,
	}
}

func (h *Hub) Handle(conn *websocket.Conn) {
	client := &hubClient{conn: conn, send: make(chan []byte, maxSendBacklog)}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	go client.writeLoop()
	client.readLoop(h)
}

func (h *Hub) Publish(_ context.Context, event Event) error {
	h.Broadcast(event)
	return nil
}

func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	clients := make([]*hubClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	batchSize := h.batchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	for start := 0; start < len(clients); start += batchSize {
		end := start + batchSize
		if end > len(clients) {
			end = len(clients)
		}
		h.broadcastBatch(data, clients[start:end])
		if h.onBatch != nil {
			h.onBatch(end - start)
		}
		if end < len(clients) && h.batchDelay > 0 {
			time.Sleep(h.batchDelay)
		}
	}
}

func (h *Hub) broadcastBatch(data []byte, clients []*hubClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for _, client := range clients {
		if _, ok := h.clients[client]; !ok {
			continue
		}
		select {
		case client.send <- data:
		default:
			delete(h.clients, client)
			close(client.send)
			if client.conn != nil {
				_ = client.conn.Close()
			}
		}
	}
}

func (h *Hub) remove(client *hubClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for client := range h.clients {
		delete(h.clients, client)
		close(client.send)
		if client.conn != nil {
			_ = client.conn.Close()
		}
	}
}

func (c *hubClient) readLoop(h *Hub) {
	defer func() {
		h.remove(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *hubClient) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
