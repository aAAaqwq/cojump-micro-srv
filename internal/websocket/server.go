package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yourusername/cojump-micro-srv/internal/hub"
	"github.com/yourusername/cojump-micro-srv/internal/model"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for mini-program access
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Server represents the WebSocket server
type Server struct {
	hub    *hub.Hub
	server *http.Server
}

// NewServer creates a new WebSocket server instance
func NewServer(address string, h *hub.Hub) *Server {
	mux := http.NewServeMux()
	s := &Server{
		hub: h,
		server: &http.Server{
			Addr:    address,
			Handler: mux,
		},
	}

	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	return s
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	log.Printf("WebSocket server listening on %s", s.server.Addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()
	return nil
}

// Stop stops the WebSocket server
func (s *Server) Stop() error {
	return s.server.Close()
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleWebSocket handles WebSocket connection requests
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &hub.Client{
		ID:   clientID,
		Send: make(chan []byte, 256),
	}

	s.hub.Register(client)
	log.Printf("WebSocket client connected: %s from %s", clientID, r.RemoteAddr)

	// Start goroutines for reading and writing
	go s.writePump(conn, client)
	go s.readPump(conn, client)
}

// readPump pumps messages from the WebSocket connection to the hub
func (s *Server) readPump(conn *websocket.Conn, client *hub.Client) {
	defer func() {
		s.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize * 10) // Increase for potential larger messages
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", client.ID, err)
			}
			break
		}

		// Parse incoming message
		var msg model.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error parsing message from client %s: %v", client.ID, err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		default:
			log.Printf("Unknown message type from client %s: %s", client.ID, msg.Type)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (s *Server) writePump(conn *websocket.Conn, client *hub.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
