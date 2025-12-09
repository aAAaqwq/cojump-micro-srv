package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/yourusername/cojump-micro-srv/internal/hub"
	"github.com/yourusername/cojump-micro-srv/internal/model"
)

// Server represents the TCP server
type Server struct {
	address  string
	hub      *hub.Hub
	listener net.Listener
}

// NewServer creates a new TCP server instance
func NewServer(address string, h *hub.Hub) *Server {
	return &Server{
		address: address,
		hub:     h,
	}
}

// Start starts the TCP server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	s.listener = listener

	log.Printf("TCP server listening on %s", s.address)

	go s.acceptConnections()
	return nil
}

// Stop stops the TCP server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// acceptConnections accepts incoming TCP connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		log.Printf("New TCP connection from %s", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

// handleConnection handles individual TCP connections
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	var deviceID string

	for {
		// Set read deadline to detect dead connections
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		// Read line-delimited JSON data
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("Error reading from %s: %v", conn.RemoteAddr(), err)
			}
			break
		}

		// Parse message
		var msg model.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("Error parsing message from %s: %v", conn.RemoteAddr(), err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		case model.MessageTypeEMGData:
			// Parse EMG data
			dataBytes, err := json.Marshal(msg.Data)
			if err != nil {
				log.Printf("Error marshaling EMG data: %v", err)
				continue
			}

			var emgData model.EMGData
			if err := json.Unmarshal(dataBytes, &emgData); err != nil {
				log.Printf("Error parsing EMG data from %s: %v", conn.RemoteAddr(), err)
				continue
			}

			// Register device on first EMG data (use remote address as device ID)
			if deviceID == "" {
				deviceID = conn.RemoteAddr().String()
				device := &hub.ESP32Device{
					DeviceID: deviceID,
					Conn:     conn,
				}
				s.hub.RegisterDevice(device)
			}

			log.Printf("Received EMG data from device %s: %d values",
				deviceID, len(emgData.Values))

			// Broadcast to WebSocket clients
			s.hub.BroadcastEMG(&emgData)

			// Send acknowledgment back to ESP32
			ack := []byte("OK\n")
			if _, err := conn.Write(ack); err != nil {
				log.Printf("Error sending ACK to %s: %v", conn.RemoteAddr(), err)
				goto cleanup
			}

		default:
			log.Printf("Unknown message type from %s: %s", conn.RemoteAddr(), msg.Type)
		}
	}

cleanup:
	if deviceID != "" {
		s.hub.UnregisterDevice(deviceID)
	}
	log.Printf("TCP connection from %s closed", conn.RemoteAddr())
}
