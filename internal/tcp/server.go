package tcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/yourusername/cojump-micro-srv/internal/hub"
	"github.com/yourusername/cojump-micro-srv/internal/model"
)

const (
	// 读缓冲区大小 - 64KB 可以容纳更多数据批次
	readBufferSize = 64 * 1024
	// 读超时时间
	readTimeout = 5 * time.Minute
)

// Server represents the TCP server
type Server struct {
	address  string
	hub      *hub.Hub
	listener net.Listener

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	conns  map[net.Conn]struct{}
	connMu sync.Mutex
}

// NewServer creates a new TCP server instance
func NewServer(address string, h *hub.Hub) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		address: address,
		hub:     h,
		ctx:     ctx,
		cancel:  cancel,
		conns:   make(map[net.Conn]struct{}),
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

// Stop stops the TCP server gracefully
func (s *Server) Stop() error {
	// Signal all goroutines to stop
	s.cancel()

	// Close listener to stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all active connections
	s.connMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.connMu.Unlock()

	// Wait for all connection handlers to finish
	s.wg.Wait()
	log.Println("TCP server stopped")
	return nil
}

// trackConn adds a connection to tracking
func (s *Server) trackConn(conn net.Conn) {
	s.connMu.Lock()
	s.conns[conn] = struct{}{}
	s.connMu.Unlock()
}

// untrackConn removes a connection from tracking
func (s *Server) untrackConn(conn net.Conn) {
	s.connMu.Lock()
	delete(s.conns, conn)
	s.connMu.Unlock()
}

// acceptConnections accepts incoming TCP connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // Server is shutting down
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		log.Printf("New TCP connection from %s", conn.RemoteAddr())
		s.trackConn(conn)
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles individual TCP connections
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		s.untrackConn(conn)
		s.wg.Done()
	}()

	// 设置 TCP 连接选项，禁用 Nagle 算法以减少延迟
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	// 使用更大的缓冲区提高读取效率
	reader := bufio.NewReaderSize(conn, readBufferSize)

	var deviceID string
	remoteAddr := conn.RemoteAddr().String()

	// 预分配 ACK 字节切片
	ack := []byte("OK\n")

	// 只在循环外设置一次超时（每次读取成功后更新）
	conn.SetReadDeadline(time.Now().Add(readTimeout))

	// 性能监控计数器
	var msgCount uint64
	lastLogTime := time.Now()

	for {
		// Read line-delimited JSON data
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("Connection timeout from %s", remoteAddr)
			} else if err.Error() != "EOF" {
				log.Printf("Error reading from %s: %v", remoteAddr, err)
			}
			break
		}

		// 读取成功后更新超时
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		// 直接解析到目标结构体，避免二次序列化
		var msg model.EMGMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			log.Printf("Error parsing message from %s: %v", remoteAddr, err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		case model.MessageTypeEMGData:
			// Register device on first EMG data
			if deviceID == "" {
				deviceID = remoteAddr
				device := &hub.ESP32Device{
					DeviceID: deviceID,
					Conn:     conn,
				}
				s.hub.RegisterDevice(device)
				log.Printf("Device %s registered", deviceID)
			}

			// Broadcast to WebSocket clients (非阻塞)
			s.hub.BroadcastEMG(&msg.Data)

			// 直接写入 ACK，不使用 bufio，避免 Flush 阻塞
			if _, err := conn.Write(ack); err != nil {
				log.Printf("Error sending ACK to %s: %v", remoteAddr, err)
				goto cleanup
			}

			// 性能监控：每10秒输出一次统计
			msgCount++
			if time.Since(lastLogTime) >= 10*time.Second {
				log.Printf("Device %s: received %d messages in last 10s (%.1f msg/s)",
					deviceID, msgCount, float64(msgCount)/10.0)
				msgCount = 0
				lastLogTime = time.Now()
			}

		default:
			log.Printf("Unknown message type from %s: %s", remoteAddr, msg.Type)
		}
	}

cleanup:
	if deviceID != "" {
		s.hub.UnregisterDevice(deviceID)
	}
	log.Printf("TCP connection from %s closed", remoteAddr)
}
