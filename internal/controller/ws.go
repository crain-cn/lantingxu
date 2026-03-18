package controller

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 64),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.Close()
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				_ = c.WriteMessage(websocket.TextMessage, msg)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

var tickerHub *Hub

func SetTickerHub(h *Hub) { tickerHub = h }

func BroadcastTicker(text string) {
	if tickerHub != nil {
		tickerHub.Broadcast([]byte(text))
	}
}

func HandleWS(w http.ResponseWriter, r *http.Request) {
	if tickerHub == nil {
		http.Error(w, "ticker not configured", http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	tickerHub.register <- conn
	defer func() { tickerHub.unregister <- conn }()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
