package hub

import (
	"encoding/json"
	"log"
	"net"
	"sync"

	"github.com/yourusername/cojump-micro-srv/internal/model"
)

// Client represents a WebSocket client connection
type Client struct {
	ID   string
	Send chan []byte
}

// ESP32Device represents an ESP32 TCP connection
type ESP32Device struct {
	DeviceID string
	Conn     net.Conn
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	// WebSocket clients
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client

	// ESP32 devices
	devices       map[string]*ESP32Device
	registerDev   chan *ESP32Device
	unregisterDev chan string

	// Communication channels
	broadcastEMG chan *model.EMGData

	// Shutdown
	done chan struct{}

	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		devices:       make(map[string]*ESP32Device),
		registerDev:   make(chan *ESP32Device),
		unregisterDev: make(chan string),
		broadcastEMG:  make(chan *model.EMGData, 256),
		done:          make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			// Close all client channels
			h.mu.Lock()
			for client := range h.clients {
				close(client.Send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			log.Println("Hub stopped")
			return

		// WebSocket client registration
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client registered: %s, total clients: %d", client.ID, len(h.clients))

		// WebSocket client unregistration
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.mu.Lock()
				delete(h.clients, client)
				close(client.Send)
				h.mu.Unlock()
				log.Printf("WebSocket client unregistered: %s, total clients: %d", client.ID, len(h.clients))
			}

		// ESP32 device registration
		case device := <-h.registerDev:
			h.mu.Lock()
			h.devices[device.DeviceID] = device
			h.mu.Unlock()
			log.Printf("ESP32 device registered: %s, total devices: %d", device.DeviceID, len(h.devices))

		// ESP32 device unregistration
		case deviceID := <-h.unregisterDev:
			if _, ok := h.devices[deviceID]; ok {
				h.mu.Lock()
				delete(h.devices, deviceID)
				h.mu.Unlock()
				log.Printf("ESP32 device unregistered: %s, total devices: %d", deviceID, len(h.devices))
			}

		// Broadcast EMG data to WebSocket clients
		case emgData := <-h.broadcastEMG:
			msg := model.Message{
				Type: model.MessageTypeEMGData,
				Data: emgData,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshaling EMG data: %v", err)
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- data:
				default:
					close(client.Send)
					delete(h.clients, client)
					log.Printf("WebSocket client %s send buffer full, disconnecting", client.ID)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a WebSocket client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a WebSocket client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// RegisterDevice adds an ESP32 device to the hub
func (h *Hub) RegisterDevice(device *ESP32Device) {
	h.registerDev <- device
}

// UnregisterDevice removes an ESP32 device from the hub
func (h *Hub) UnregisterDevice(deviceID string) {
	h.unregisterDev <- deviceID
}

// BroadcastEMG sends EMG data to all connected WebSocket clients
// 非阻塞模式：如果 channel 满了就丢弃数据，避免阻塞 TCP 读取
func (h *Hub) BroadcastEMG(data *model.EMGData) {
	select {
	case h.broadcastEMG <- data:
		// 成功发送
	default:
		// channel 满了，丢弃数据但不阻塞
		log.Println("WARNING: broadcastEMG channel full, dropping EMG data")
	}
}

// GetClientCount returns the number of connected WebSocket clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetDeviceCount returns the number of connected ESP32 devices
func (h *Hub) GetDeviceCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.devices)
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	close(h.done)
}
