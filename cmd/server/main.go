package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/cojump-micro-srv/internal/hub"
	"github.com/yourusername/cojump-micro-srv/internal/tcp"
	"github.com/yourusername/cojump-micro-srv/internal/websocket"
)

var (
	tcpAddr = flag.String("tcp", ":8081", "TCP server address for ESP32")
	wsAddr  = flag.String("ws", ":8082", "WebSocket server address for mini-program")
)

func main() {
	flag.Parse()

	log.Println("Starting Cojump Micro Service...")

	// Create hub for data forwarding
	h := hub.NewHub()
	go h.Run()
	log.Println("Hub started")

	// Start TCP server for ESP32
	tcpServer := tcp.NewServer(*tcpAddr, h)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	// Start WebSocket server for mini-program
	wsServer := websocket.NewServer(*wsAddr, h)
	if err := wsServer.Start(); err != nil {
		log.Fatalf("Failed to start WebSocket server: %v", err)
	}

	log.Println("All services started successfully")
	log.Printf("TCP server: %s (for ESP32)", *tcpAddr)
	log.Printf("WebSocket server: %s (for mini-program)", *wsAddr)
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down servers...")

	// Graceful shutdown
	if err := tcpServer.Stop(); err != nil {
		log.Printf("Error stopping TCP server: %v", err)
	}

	if err := wsServer.Stop(); err != nil {
		log.Printf("Error stopping WebSocket server: %v", err)
	}

	log.Println("Servers stopped. Goodbye!")
}
