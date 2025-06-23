package main

import (
	"log"
	"time"

	"./mcxapi" // Change this to the correct import path, e.g. "./mcxapi" if it's a local package
)

type MDIConfig struct {
	Address            string
	Port               int
	IsEMDI             bool
	Service            string
	Channel            string
	SnapshotEnabled    bool
	IncrementalEnabled bool
}

func main() {
	config := MDIConfig{
		Address:            "239.100.100.100",
		Port:               50000,
		IsEMDI:             true,
		Service:            "A",
		Channel:            "1",
		SnapshotEnabled:    true,
		IncrementalEnabled: true,
	}
	handler := &ExampleHandler{}

	client := mcxapi.NewMDIClient(config, handler)

	if err := client.Connect(); err != nil {
		log.Fatalf("Connection error: %v", err)
	}

	// Run for a while
	time.Sleep(5 * time.Minute)
	client.Disconnect()
}

type ExampleHandler struct{}

func (h *ExampleHandler) OnDepthSnapshot(snapshot *mcxapi.DepthSnapshot) {
	log.Printf("Received snapshot for instrument %d", snapshot.SecurityID)
}

func (h *ExampleHandler) OnDepthIncremental(incremental *mcxapi.DepthIncremental) {
	for _, entry := range incremental.Entries {
		log.Printf("Incremental update: %v @ %v", entry.MDEntrySize, entry.MDEntryPx)
	}
}

func (h *ExampleHandler) OnProductStateChange(state *mcxapi.ProductStateChange) {
	log.Printf("Product state changed to %d", state.TradSesStatus)
}

func (h *ExampleHandler) OnIndexStats(stats *mcxapi.IndexStats) {
	log.Printf("Index update: %.2f (H: %.2f L: %.2f)", stats.IndexValue, stats.IndexHigh, stats.IndexLow)
}

func (h *ExampleHandler) OnError(err error) {
	log.Printf("Error: %v", err)
}
