package model

// Message represents the WebSocket protocol message format
// Protocol: type:<message_type>, data:<data_content>
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EMGMessage 是专用于 EMG 数据的消息结构体，避免二次 JSON 序列化
type EMGMessage struct {
	Type string  `json:"type"`
	Data EMGData `json:"data"`
}

// EMGData represents EMG sensor data from ESP32
type EMGData struct {
	Values []int `json:"values"` // EMG sensor values (int array)
}

// Message types
const (
	MessageTypeEMGData = "emg"
)
