package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats:4222"
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Unable to connect to NATS: %v\n", err)
	}
	defer nc.Close()

	_, err = nc.Subscribe("job.completed", func(m *nats.Msg) {
		log.Printf("🔔 NOTIFICATION: Job %s has been successfully completed!", string(m.Data))
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to job.completed: %v", err)
	}

	_, err = nc.Subscribe("job.failed", func(m *nats.Msg) {
		log.Printf("🚨 NOTIFICATION: Job %s has failed!", string(m.Data))
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to job.failed: %v", err)
	}

	log.Println("Notification service is listening for events...")

	// Wait for shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Notification service shutting down")
}
