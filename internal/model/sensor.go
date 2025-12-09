package model

// Message represents the WebSocket protocol message format
// Protocol: type:<message_type>, data:<data_content>
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EMGData represents EMG sensor data from ESP32
type EMGData struct {
	Values    []int     `json:"values"` // EMG sensor values (int array)
}

// Message types
const (
	MessageTypeEMGData = "emg"
)
