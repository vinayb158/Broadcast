package listener

import (
	"fmt"
	"log"
	"net"
	"time"
)

// Config
const (
	multicastIP = "233.1.2.5" // replace with actual MCX multicast group
	port        = 5001        // replace with the actual port
	ifaceName   = "eth0"      // your network interface (e.g., "en0" on mac, "eth0" on Linux)
	bufSize     = 9000        // large enough for T7 EMDI packets
)

func main() {
	addr := net.UDPAddr{
		IP:   net.ParseIP(multicastIP),
		Port: port,
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		log.Fatalf("Failed to get interface %s: %v", ifaceName, err)
	}

	conn, err := net.ListenMulticastUDP("udp", iface, &addr)
	if err != nil {
		log.Fatalf("Failed to join multicast group: %v", err)
	}
	defer conn.Close()

	conn.SetReadBuffer(bufSize)
	log.Printf("Listening on %s:%d via interface %s\n", multicastIP, port, ifaceName)

	buffer := make([]byte, bufSize)

	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			continue
		}

		timestamp := time.Now().Format("15:04:05.000")
		log.Printf("[%s] Packet from %s, %d bytes", timestamp, src, n)

		// Print raw hex (optional, for debugging)
		fmt.Printf("% X\n", buffer[:n])

		// TODO: Pass `buffer[:n]` to your FAST/FIX decoder
	}
}
