package tcp

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/yourusername/cojump-micro-srv/internal/hub"
	"github.com/yourusername/cojump-micro-srv/internal/model"
)

func TestTCPServer_HandleSingleEMGData(t *testing.T) {
	// Create hub and server
	h := hub.NewHub()
	go h.Run()

	server := NewServer(":18080", h)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as ESP32 device
	conn, err := net.Dial("tcp", "localhost:18080")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send EMG data
	emgData := model.EMGData{
		// DeviceID:  "esp32-001",
		// Timestamp: time.Now(),
		Values:    []int{100, 200, 300, 400, 500},
	}

	msg := model.Message{
		Type: model.MessageTypeEMGData,
		Data: emgData,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Send data with newline
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read acknowledgment
	reader := bufio.NewReader(conn)
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read ACK: %v", err)
	}

	if ack != "OK\n" {
		t.Errorf("Expected 'OK\\n', got '%s'", ack)
	}

	// Verify device is registered
	time.Sleep(50 * time.Millisecond)
	if h.GetDeviceCount() != 1 {
		t.Errorf("Expected 1 device, got %d", h.GetDeviceCount())
	}
}

func TestTCPServer_HandleBatchEMGData(t *testing.T) {
	// Create hub and server
	h := hub.NewHub()
	go h.Run()

	server := NewServer(":18081", h)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Connect as ESP32 device
	conn, err := net.Dial("tcp", "localhost:18081")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send multiple batches of EMG data
	batchCount := 10
	samplesPerBatch := 100

	for i := 0; i < batchCount; i++ {
		// Generate batch data
		values := make([]int, samplesPerBatch)
		for j := 0; j < samplesPerBatch; j++ {
			values[j] = 100 + j + (i * 10)
		}

		emgData := model.EMGData{
			// DeviceID:  "esp32-batch-001",
			// Timestamp: time.Now(),
			Values:    values,
		}

		msg := model.Message{
			Type: model.MessageTypeEMGData,
			Data: emgData,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Batch %d: Failed to marshal message: %v", i, err)
		}

		// Send data with newline
		data = append(data, '\n')
		if _, err := conn.Write(data); err != nil {
			t.Fatalf("Batch %d: Failed to write data: %v", i, err)
		}

		// Read acknowledgment
		ack, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Batch %d: Failed to read ACK: %v", i, err)
		}

		if ack != "OK\n" {
			t.Errorf("Batch %d: Expected 'OK\\n', got '%s'", i, ack)
		}
	}

	// Verify device is still registered
	time.Sleep(50 * time.Millisecond)
	if h.GetDeviceCount() != 1 {
		t.Errorf("Expected 1 device, got %d", h.GetDeviceCount())
	}
}

func TestTCPServer_HandleLargeEMGData(t *testing.T) {
	// Create hub and server
	h := hub.NewHub()
	go h.Run()

	server := NewServer(":18082", h)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Connect as ESP32 device
	conn, err := net.Dial("tcp", "localhost:18082")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send large batch (1000 samples)
	largeValues := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		largeValues[i] = i * 2
	}

	emgData := model.EMGData{
		// DeviceID:  "esp32-large-001",
		// Timestamp: time.Now(),
		Values:    largeValues,
	}

	msg := model.Message{
		Type: model.MessageTypeEMGData,
		Data: emgData,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Send data with newline
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read acknowledgment
	reader := bufio.NewReader(conn)
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read ACK: %v", err)
	}

	if ack != "OK\n" {
		t.Errorf("Expected 'OK\\n', got '%s'", ack)
	}
}

func TestTCPServer_MultipleDevices(t *testing.T) {
	// Create hub and server
	h := hub.NewHub()
	go h.Run()

	server := NewServer(":18083", h)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Connect multiple ESP32 devices
	deviceCount := 5
	conns := make([]net.Conn, deviceCount)

	for i := 0; i < deviceCount; i++ {
		conn, err := net.Dial("tcp", "localhost:18083")
		if err != nil {
			t.Fatalf("Device %d: Failed to connect: %v", i, err)
		}
		defer conn.Close()
		conns[i] = conn

		// Send initial EMG data to register device
		emgData := model.EMGData{
			// DeviceID:  "esp32-multi-" + string(rune('A'+i)),
			// Timestamp: time.Now(),
			Values:    []int{i * 100, i * 200, i * 300},
		}

		msg := model.Message{
			Type: model.MessageTypeEMGData,
			Data: emgData,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Device %d: Failed to marshal message: %v", i, err)
		}

		data = append(data, '\n')
		if _, err := conn.Write(data); err != nil {
			t.Fatalf("Device %d: Failed to write data: %v", i, err)
		}

		// Read ACK
		reader := bufio.NewReader(conn)
		ack, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Device %d: Failed to read ACK: %v", i, err)
		}

		if ack != "OK\n" {
			t.Errorf("Device %d: Expected 'OK\\n', got '%s'", i, ack)
		}
	}

	// Verify all devices are registered
	time.Sleep(100 * time.Millisecond)
	if h.GetDeviceCount() != deviceCount {
		t.Errorf("Expected %d devices, got %d", deviceCount, h.GetDeviceCount())
	}
}

func TestTCPServer_InvalidMessageFormat(t *testing.T) {
	// Create hub and server
	h := hub.NewHub()
	go h.Run()

	server := NewServer(":18084", h)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Connect as ESP32 device
	conn, err := net.Dial("tcp", "localhost:18084")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send invalid JSON
	invalidData := []byte("invalid json data\n")
	if _, err := conn.Write(invalidData); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Server should not crash, connection should remain open
	time.Sleep(50 * time.Millisecond)

	// Send valid data after invalid data
	emgData := model.EMGData{
		// DeviceID:  "esp32-recovery-001",
		// Timestamp: time.Now(),
		Values:    []int{100, 200, 300},
	}

	msg := model.Message{
		Type: model.MessageTypeEMGData,
		Data: emgData,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read acknowledgment
	reader := bufio.NewReader(conn)
	ack, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read ACK: %v", err)
	}

	if ack != "OK\n" {
		t.Errorf("Expected 'OK\\n', got '%s'", ack)
	}
}
