package multicast

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	multicastAddress = "224.0.0.1:5005"   // Multicast address and port
	maxDatagramSize  = 8192               // Max packet size
	interfaceName    = ""                 // Network interface (empty for default)
	outputDir        = "received_packets" // Directory to save packets
)

func main() {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	log.Println("Starting multicast UDP listener...")

	// Resolve multicast address
	addr, err := net.ResolveUDPAddr("udp", multicastAddress)
	if err != nil {
		log.Fatalf("Error resolving address: %v", err)
	}

	// Get network interface
	var iface *net.Interface
	if interfaceName != "" {
		iface, err = net.InterfaceByName(interfaceName)
		if err != nil {
			log.Fatalf("Error getting interface %s: %v", interfaceName, err)
		}
	}

	// Create multicast connection
	conn, err := net.ListenMulticastUDP("udp", iface, addr)
	if err != nil {
		log.Fatalf("Error setting up multicast listener: %v", err)
	}
	defer conn.Close()

	if err := conn.SetReadBuffer(maxDatagramSize); err != nil {
		log.Printf("Warning: Error setting read buffer size: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down multicast listener...")
		conn.Close()
		os.Exit(0)
	}()

	log.Printf("Listening for multicast packets on %s", multicastAddress)
	log.Printf("Saving packets to directory: %s", outputDir)

	buffer := make([]byte, maxDatagramSize)

	for {
		// Read packet
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		packet := buffer[:n]
		log.Printf("Received %d bytes from %s", n, src.String())

		// Save packet to file
		if err := savePacketToFile(packet); err != nil {
			log.Printf("Error saving packet: %v", err)
		}
	}
}

func savePacketToFile(data []byte) error {
	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405.000")
	filename := filepath.Join(outputDir, fmt.Sprintf("packet_%s.bin", timestamp))

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Write data to file
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	log.Printf("Packet saved to %s", filename)
	return nil
}
