package hub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yourusername/cojump-micro-srv/internal/model"
)

func TestHub_RegisterClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		ID:   "test-client-001",
		Send: make(chan []byte, 256),
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	if h.GetClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", h.GetClientCount())
	}
}

func TestHub_UnregisterClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		ID:   "test-client-002",
		Send: make(chan []byte, 256),
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.Unregister(client)
	time.Sleep(50 * time.Millisecond)

	if h.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", h.GetClientCount())
	}
}

func TestHub_BroadcastEMG_SingleClient(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		ID:   "test-client-003",
		Send: make(chan []byte, 256),
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Send EMG data
	emgData := &model.EMGData{
		Values: []int{100, 200, 300, 400, 500},
	}

	h.BroadcastEMG(emgData)

	// Receive data from client channel
	select {
	case data := <-client.Send:
		var msg model.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if msg.Type != model.MessageTypeEMGData {
			t.Errorf("Expected message type '%s', got '%s'", model.MessageTypeEMGData, msg.Type)
		}

		// Verify data content
		dataBytes, _ := json.Marshal(msg.Data)
		var receivedData model.EMGData
		if err := json.Unmarshal(dataBytes, &receivedData); err != nil {
			t.Fatalf("Failed to unmarshal EMG data: %v", err)
		}

		if len(receivedData.Values) != len(emgData.Values) {
			t.Errorf("Expected %d values, got %d", len(emgData.Values), len(receivedData.Values))
		}

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcast message")
	}
}

func TestHub_BroadcastEMG_MultipleClients(t *testing.T) {
	h := NewHub()
	go h.Run()

	clientCount := 5
	clients := make([]*Client, clientCount)

	// Register multiple clients
	for i := 0; i < clientCount; i++ {
		clients[i] = &Client{
			ID:   "test-client-multi-" + string(rune('A'+i)),
			Send: make(chan []byte, 256),
		}
		h.Register(clients[i])
	}
	time.Sleep(100 * time.Millisecond)

	// Send EMG data
	emgData := &model.EMGData{
		Values: []int{10, 20, 30, 40, 50},
	}

	h.BroadcastEMG(emgData)

	// Verify all clients received the data
	for i, client := range clients {
		select {
		case data := <-client.Send:
			var msg model.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("Client %d: Failed to unmarshal message: %v", i, err)
			}

			if msg.Type != model.MessageTypeEMGData {
				t.Errorf("Client %d: Expected message type '%s', got '%s'", i, model.MessageTypeEMGData, msg.Type)
			}

		case <-time.After(1 * time.Second):
			t.Fatalf("Client %d: Timeout waiting for broadcast message", i)
		}
	}
}

func TestHub_BroadcastEMG_LargeBatch(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		ID:   "test-client-large-001",
		Send: make(chan []byte, 256),
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Send large batch of EMG data (1000 samples)
	largeValues := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		largeValues[i] = i
	}

	emgData := &model.EMGData{
		Values: largeValues,
	}

	h.BroadcastEMG(emgData)

	// Receive and verify data
	select {
	case data := <-client.Send:
		var msg model.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		dataBytes, _ := json.Marshal(msg.Data)
		var receivedData model.EMGData
		if err := json.Unmarshal(dataBytes, &receivedData); err != nil {
			t.Fatalf("Failed to unmarshal EMG data: %v", err)
		}

		if len(receivedData.Values) != 1000 {
			t.Errorf("Expected 1000 values, got %d", len(receivedData.Values))
		}

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for broadcast message")
	}
}

func TestHub_RegisterDevice(t *testing.T) {
	h := NewHub()
	go h.Run()

	device := &ESP32Device{
		DeviceID: "esp32-device-001",
		Conn:     nil, // Mock connection
	}

	h.RegisterDevice(device)
	time.Sleep(50 * time.Millisecond)

	if h.GetDeviceCount() != 1 {
		t.Errorf("Expected 1 device, got %d", h.GetDeviceCount())
	}
}

func TestHub_UnregisterDevice(t *testing.T) {
	h := NewHub()
	go h.Run()

	device := &ESP32Device{
		DeviceID: "esp32-device-002",
		Conn:     nil,
	}

	h.RegisterDevice(device)
	time.Sleep(50 * time.Millisecond)

	h.UnregisterDevice(device.DeviceID)
	time.Sleep(50 * time.Millisecond)

	if h.GetDeviceCount() != 0 {
		t.Errorf("Expected 0 devices, got %d", h.GetDeviceCount())
	}
}

func TestHub_MultipleBroadcasts(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		ID:   "test-client-multiple-001",
		Send: make(chan []byte, 256),
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Send multiple EMG data packets
	broadcastCount := 100
	for i := 0; i < broadcastCount; i++ {
		emgData := &model.EMGData{
			Values: []int{i, i * 2, i * 3},
		}
		h.BroadcastEMG(emgData)
	}

	// Verify all broadcasts were received
	receivedCount := 0
	timeout := time.After(5 * time.Second)

	for receivedCount < broadcastCount {
		select {
		case <-client.Send:
			receivedCount++
		case <-timeout:
			t.Fatalf("Timeout: Received %d broadcasts, expected %d", receivedCount, broadcastCount)
		}
	}

	if receivedCount != broadcastCount {
		t.Errorf("Expected %d broadcasts, got %d", broadcastCount, receivedCount)
	}
}

func TestHub_ConcurrentOperations(t *testing.T) {
	h := NewHub()
	go h.Run()

	// Register multiple clients concurrently
	clientCount := 10
	clients := make([]*Client, clientCount)

	for i := 0; i < clientCount; i++ {
		go func(idx int) {
			clients[idx] = &Client{
				ID:   "test-client-concurrent-" + string(rune('0'+idx)),
				Send: make(chan []byte, 256),
			}
			h.Register(clients[idx])
		}(i)
	}

	time.Sleep(200 * time.Millisecond)

	// Send broadcasts concurrently
	for i := 0; i < 50; i++ {
		go func(idx int) {
			emgData := &model.EMGData{
				Values: []int{idx, idx * 2, idx * 3},
			}
			h.BroadcastEMG(emgData)
		}(i)
	}

	time.Sleep(500 * time.Millisecond)

	// Verify clients are still registered
	if h.GetClientCount() != clientCount {
		t.Errorf("Expected %d clients, got %d", clientCount, h.GetClientCount())
	}
}
