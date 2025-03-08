package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/pierrec/lz4/v4"
	"golang.org/x/net/ipv4"
)

type MSG_HDR struct {
	Res1          [12]byte // 12 bytes (8 RESERVE + 4 RESERVE)
	LogTime       uint32   // 4 bytes
	Alphachar     [2]byte  // 2 bytes
	TrCode        uint16   // 2 bytes
	ErrCode       uint16   // 2 bytes
	BCSeqNo       int32    // 4 bytes
	Reserved      uint32   // 4 bytes
	TimeStamp     [8]byte  // 8 bytes
	Filler2       [8]byte  // 8 bytes
	MessageLength uint16   // 2 bytes
} //
// MBPP struct (Size: 14 bytes)
type MBPP struct {
	Quantity       int64  // 8 bytes
	Price          uint32 // 4 bytes
	NumberOfOrders uint16 // 2 bytes
	BbBuySellFlag  uint16 // 2 bytes
}
type MBP struct {
	Token                          int32    // 4 bytes
	BookType                       uint16   // 2 bytes
	TradingStatus                  uint16   // 2 bytes
	VolumeTradedToday              int64    // 8 bytes
	LastTradedPrice                uint32   // 4 bytes
	NetChangeIndicator             byte     // 1 byte
	Ch                             byte     // 1 byte (Manipulation)
	NetPriceChangeFromClosingPrice uint32   // 4 bytes
	LastTradedQuantity             uint32   // 4 bytes
	LastTradedTime                 uint32   // 4 bytes
	AverageTradedPrice             uint32   // 4 bytes
	AuctionNumber                  uint16   // 2 bytes
	AuctionStatus                  uint16   // 2 bytes
	InitiatorType                  uint16   // 2 bytes
	InitiatorPrice                 uint32   // 4 bytes
	InitiatorQuantity              uint32   // 4 bytes
	AuctionPrice                   uint32   // 4 bytes
	AuctionQuantity                uint32   // 4 bytes
	Mbpp                           [10]MBPP // 10 * 12 bytes = 120 bytes
	BbTotalBuyFlag                 uint16   // 2 bytes
	BbTotalSellFlag                uint16   // 2 bytes
	TBQ                            int64    // 8 bytes
	TSQ                            int64    // 8 bytes
	MBPIndicator                   byte     // 1 byte
	Res                            byte     // 1 byte
	ClosingPrice                   int32    // 4 bytes
	OpenPrice                      int32    // 4 bytes
	HighPrice                      int32    // 4 bytes
	LowPrice                       int32    // 4 bytes
	IndiClosePrice                 int32    // 4 bytes
}

type MBPFO struct {
	Token                          uint32   // 4 bytes
	BookType                       uint16   // 2 bytes
	TradingStatus                  uint16   // 2 bytes
	VolumeTradedToday              uint32   // 4 bytes
	LastTradedPrice                uint32   // 4 bytes
	NetChangeIndicator             byte     // 1 byte
	Ch                             byte     // 1 byte (Manipulation)
	NetPriceChangeFromClosingPrice uint32   // 4 bytes
	LastTradedQuantity             uint32   // 4 bytes
	LastTradedTime                 uint32   // 4 bytes
	AverageTradedPrice             uint32   // 4 bytes
	AuctionNumber                  uint16   // 2 bytes
	AuctionStatus                  uint16   // 2 bytes
	InitiatorType                  uint16   // 2 bytes
	InitiatorPrice                 uint32   // 4 bytes
	InitiatorQuantity              uint32   // 4 bytes
	AuctionPrice                   uint32   // 4 bytes
	AuctionQuantity                uint32   // 4 bytes
	Mbpp                           [10]MBPP // 10 * 12 bytes = 120 bytes
	BbTotalBuyFlag                 uint16   // 2 bytes
	BbTotalSellFlag                uint16   // 2 bytes
	TBQ                            float64  // 8 bytes
	TSQ                            float64  // 8 bytes
	Buyi                           byte     // 1 byte
	MBPIndicator                   byte     // 1 byte
	ClosingPrice                   int32    // 4 bytes
	OpenPrice                      int32    // 4 bytes
	HighPrice                      int32    // 4 bytes
	LowPrice                       int32    // 4 bytes
} // Total: 212 bytes

// MBPCUR struct (Size: 212 bytes)
type MBPCUR struct {
	Token                          uint32   // 4 bytes
	BookType                       uint16   // 2 bytes
	TradingStatus                  uint16   // 2 bytes
	VolumeTradedToday              uint32   // 4 bytes
	LastTradedPrice                uint32   // 4 bytes
	NetChangeIndicator             byte     // 1 byte
	NetPriceChangeFromClosingPrice uint32   // 4 bytes
	LastTradedQuantity             uint32   // 4 bytes
	LastTradedTime                 uint32   // 4 bytes
	AverageTradedPrice             uint32   // 4 bytes
	AuctionNumber                  uint16   // 2 bytes
	AuctionStatus                  uint16   // 2 bytes
	InitiatorType                  uint16   // 2 bytes
	InitiatorPrice                 uint32   // 4 bytes
	InitiatorQuantity              uint32   // 4 bytes
	AuctionPrice                   uint32   // 4 bytes
	AuctionQuantity                uint32   // 4 bytes
	Mbpp                           [10]MBPP // 10 * 12 bytes = 120 bytes
	BbTotalBuyFlag                 uint16   // 2 bytes
	BbTotalSellFlag                uint16   // 2 bytes
	TBQ                            float64  // 8 bytes
	TSQ                            float64  // 8 bytes
	MBPIndicator                   [2]byte  // 2 bytes
	ClosingPrice                   int32    // 4 bytes
	OpenPrice                      int32    // 4 bytes
	HighPrice                      int32    // 4 bytes
	LowPrice                       int32    // 4 bytes
}

func main() {
	localIP := "192.168.1.8"   // Replace with your actual local IP
	multicastIP := "233.1.2.5" // Replace with your actual multicast group IP
	port := 34256

	// Resolve UDP address for binding
	localAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", localIP, port))
	if err != nil {
		fmt.Println("Error resolving local address:", err)
		os.Exit(1)
	}

	// Create UDP connection
	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		fmt.Println("Error binding to port:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Socket bound to", localAddr)

	// Find the network interface associated with localIP
	iface, err := findInterfaceByIP(localIP)
	if err != nil {
		fmt.Println("Error finding network interface:", err)
		os.Exit(1)
	}

	// Wrap connection with ipv4.PacketConn to configure multicast options
	p := ipv4.NewPacketConn(conn)

	// Join multicast group on the found interface
	group := &net.UDPAddr{IP: net.ParseIP(multicastIP)}
	if err := p.JoinGroup(iface, group); err != nil {
		fmt.Println("Error joining multicast group:", err)
		os.Exit(1)
	}

	// Enable multicast loopback (optional)
	if err := p.SetMulticastLoopback(true); err != nil {
		fmt.Println("Error setting multicast loopback:", err)
		os.Exit(1)
	}

	fmt.Println("Joined multicast group", multicastIP)

	// Start receiving data in a goroutine
	go sockDataArrival(conn)

	fmt.Println("Receiving Thread Started.")
	select {} // Keep the program running
}

// Finds the network interface associated with a given IP address
func findInterfaceByIP(ip string) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.String() == ip {
				return &iface, nil
			}
		}
	}
	return nil, fmt.Errorf("no interface found for IP %s", ip)
}

// Function to handle incoming UDP data
func sockDataArrival(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from socket:", err)
			continue
		}
		fmt.Printf("Received data from %s: %s\n", addr, string(buffer[:n]))
	}
}

// Convert MBP struct to bytes
func (m *MBP) ToBytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, m)
	return buf.Bytes(), err
}

// Convert bytes to MBP struct
func BytesToMBP(data []byte) (*MBP, error) {
	var msg MBP
	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.LittleEndian, &msg)
	return &msg, err
}
func CompressLZ4(data []byte) ([]byte, error) {
	out := make([]byte, lz4.CompressBlockBound(len(data)))
	n, err := lz4.CompressBlock(data, out, nil)
	if err != nil {
		return nil, err
	}
	return out[:n], nil
}
