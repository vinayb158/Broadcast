package mcxapi

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Constants for template IDs
const (
	TIDPacketHeaderEMDI      = 60
	TIDPacketHeaderMDI       = 65
	TIDFASTReset             = 120
	TIDDepthSnapshotEMDI     = 93
	TIDDepthSnapshotMDI      = 101
	TIDDepthIncrementalEMDI  = 94
	TIDDepthIncrementalMDI   = 102
	TIDProductStateChange    = 97
	TIDMassInstrumentState   = 99
	TIDInstrumentStateChange = 98
	TIDIndexStats            = 50
	TIDFunctionalBeacon      = 109
)

// DepthSnapshot represents a market depth snapshot message (define fields as needed)
type DepthSnapshot struct {
	// Add appropriate fields here
}

// DepthIncremental represents a market depth incremental message (define fields as needed)
type DepthIncremental struct {
	// Add appropriate fields here
}

// ProductStateChange represents a product state change message (define fields as needed)
type ProductStateChange struct {
	// Add appropriate fields here
}

// IndexStats represents index statistics message (define fields as needed)
type IndexStats struct {
	// Add appropriate fields here
}

// MarketDataHandler defines the interface for processing market data messages
type MarketDataHandler interface {
	OnDepthSnapshot(*DepthSnapshot)
	OnDepthIncremental(*DepthIncremental)
	OnProductStateChange(*ProductStateChange)
	OnIndexStats(*IndexStats)
	OnError(error)
}

// MDIConfig holds configuration for the MCX market data interface
type MDIConfig struct {
	Address            string
	Port               int
	IsEMDI             bool
	Service            string // "A" or "B" for live-live concept
	Channel            string // Product channel identifier
	SnapshotEnabled    bool
	IncrementalEnabled bool
}

// MDIClient represents a connection to MCX market data feed
type MDIClient struct {
	config       MDIConfig
	conn         *net.UDPConn
	handler      MarketDataHandler
	decoder      *Decoder
	shutdown     chan struct{}
	wg           sync.WaitGroup
	packetBuffer chan []byte
}

// NewMDIClient creates a new MCX market data client
func NewMDIClient(config MDIConfig, handler MarketDataHandler) *MDIClient {
	return &MDIClient{
		config:       config,
		handler:      handler,
		decoder:      NewDecoder(),
		shutdown:     make(chan struct{}),
		packetBuffer: make(chan []byte, 1000), // Buffer size can be adjusted
	}
}

// Connect establishes the UDP connection to the market data feed
func (c *MDIClient) Connect() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", c.config.Address, c.config.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	c.conn, err = net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %v", err)
	}

	// Set buffer sizes to handle high throughput
	if err := c.conn.SetReadBuffer(1024 * 1024 * 10); err != nil { // 10MB buffer
		log.Printf("Warning: could not set read buffer size: %v", err)
	}

	log.Printf("Connected to MCX %s feed on %s:%d (EMDI: %v)",
		c.config.Service, c.config.Address, c.config.Port, c.config.IsEMDI)

	// Start processing goroutines
	c.wg.Add(2)
	go c.receiver()
	go c.processor()

	return nil
}

// Disconnect closes the connection and stops processing
func (c *MDIClient) Disconnect() {
	close(c.shutdown)
	if c.conn != nil {
		c.conn.Close()
	}
	c.wg.Wait()
	log.Println("MCX market data client disconnected")
}

// receiver reads packets from the network and puts them in the buffer
func (c *MDIClient) receiver() {
	defer c.wg.Done()

	buffer := make([]byte, 65536) // Maximum UDP packet size

	for {
		select {
		case <-c.shutdown:
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				c.handler.OnError(fmt.Errorf("UDP read error: %v", err))
				continue
			}

			// Copy the packet data to avoid buffer reuse issues
			packet := make([]byte, n)
			copy(packet, buffer[:n])

			select {
			case c.packetBuffer <- packet:
			default:
				c.handler.OnError(errors.New("packet buffer full, dropping packet"))
			}
		}
	}
}

// processor decodes packets from the buffer
func (c *MDIClient) processor() {
	defer c.wg.Done()

	for {
		select {
		case <-c.shutdown:
			return
		case packet := <-c.packetBuffer:
			if err := c.processPacket(packet); err != nil {
				c.handler.OnError(fmt.Errorf("packet processing error: %v", err))
			}
		}
	}
}

// processPacket decodes a single UDP packet
func (c *MDIClient) processPacket(packet []byte) error {
	// Each packet contains:
	// 1. Packet header
	// 2. FAST reset message
	// 3. One or more market data messages

	// Decode packet header
	header, err := c.decoder.DecodePacketHeader(packet, c.config.IsEMDI)
	if err != nil {
		return fmt.Errorf("header decode failed: %v", err)
	}

	// Skip FAST reset message (TID 120)
	offset := len(header.PacketSeqNum) + len(header.SendingTime) + 4 // SenderCompID is 4 bytes
	if c.config.IsEMDI {
		offset += 4 + len(header.PerformanceInd) // PartitionID (4 bytes) + PerformanceInd
	}

	// The reset message is always present and has TID 120
	resetTID, resetLen := decodeTemplateID(packet[offset:])
	if resetTID != TIDFASTReset {
		return fmt.Errorf("expected FAST reset message (TID 120), got %d", resetTID)
	}
	offset += resetLen

	// Process remaining messages in the packet
	for offset < len(packet) {
		tid, tidLen := decodeTemplateID(packet[offset:])
		offset += tidLen

		msg, msgLen, err := c.decoder.DecodeMessage(packet[offset:], tid)
		if err != nil {
			return fmt.Errorf("message decode failed (TID %d): %v", tid, err)
		}
		offset += msgLen

		// Dispatch the message to the appropriate handler
		switch v := msg.(type) {
		case *DepthSnapshot:
			if c.config.SnapshotEnabled {
				c.handler.OnDepthSnapshot(v)
			}
		case *DepthIncremental:
			if c.config.IncrementalEnabled {
				c.handler.OnDepthIncremental(v)
			}
		case *ProductStateChange:
			c.handler.OnProductStateChange(v)
		case *IndexStats:
			c.handler.OnIndexStats(v)
		default:
			log.Printf("Unhandled message type: %T", v)
		}
	}

	return nil
}

// decodeTemplateID extracts the template ID from the message
func decodeTemplateID(data []byte) (uint32, int) {
	// In FAST, the template ID is stop-bit encoded
	// This is a simplified version - real implementation needs proper stop-bit decoding
	if len(data) == 0 {
		return 0, 0
	}
	return uint32(data[0] & 0x7F), 1
}

// Decoder is a stub for the FAST/MCX message decoder
type Decoder struct{}

// NewDecoder creates a new Decoder instance
func NewDecoder() *Decoder {
	return &Decoder{}
}

// PacketHeader is a stub for the decoded packet header
type PacketHeader struct {
	PacketSeqNum   []byte
	SendingTime    []byte
	PerformanceInd []byte
}

// DecodePacketHeader is a stub for decoding the packet header
func (d *Decoder) DecodePacketHeader(packet []byte, isEMDI bool) (*PacketHeader, error) {
	// Stub: return dummy header with arbitrary lengths for offset calculation
	return &PacketHeader{
		PacketSeqNum:   make([]byte, 4),
		SendingTime:    make([]byte, 8),
		PerformanceInd: make([]byte, 2),
	}, nil
}

// DecodeMessage is a stub for decoding a market data message
func (d *Decoder) DecodeMessage(data []byte, tid uint32) (interface{}, int, error) {
	// Return dummy messages based on TID for demonstration
	switch tid {
	case TIDDepthSnapshotEMDI, TIDDepthSnapshotMDI:
		return &DepthSnapshot{}, 10, nil
	case TIDDepthIncrementalEMDI, TIDDepthIncrementalMDI:
		return &DepthIncremental{}, 10, nil
	case TIDProductStateChange:
		return &ProductStateChange{}, 10, nil
	case TIDIndexStats:
		return &IndexStats{}, 10, nil
	default:
		return nil, 0, fmt.Errorf("unknown TID: %d", tid)
	}
}

// Sample usage:

// ExampleHandler implements MarketDataHandler
type ExampleHandler struct{}

func (h *ExampleHandler) OnDepthSnapshot(snapshot *DepthSnapshot) {
	log.Printf("Snapshot: %+v", snapshot)
}

func (h *ExampleHandler) OnDepthIncremental(incremental *DepthIncremental) {
	log.Printf("Incremental: %+v", incremental)
}

func (h *ExampleHandler) OnProductStateChange(state *ProductStateChange) {
	log.Printf("Product State Change: %+v", state)
}

func (h *ExampleHandler) OnIndexStats(stats *IndexStats) {
	log.Printf("Index Stats: %+v", stats)
}

func (h *ExampleHandler) OnError(err error) {
	log.Printf("Error: %v", err)
}

func main() {
	// Example configuration for EMDI Service A
	config := MDIConfig{
		Address:            "239.100.100.100", // Example multicast address
		Port:               50000,             // Example port
		IsEMDI:             true,
		Service:            "A",
		Channel:            "1", // Product channel
		SnapshotEnabled:    true,
		IncrementalEnabled: true,
	}

	handler := &ExampleHandler{}
	client := NewMDIClient(config, handler)

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Run for 60 seconds in this example
	time.Sleep(60 * time.Second)
	client.Disconnect()
}
