package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

var (
	db *pgxpool.Pool
	nc *nats.Conn
)

func initConnections() {
	var err error
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@postgres:5432/sampleapp?sslmode=disable"
	}
	db, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats:4222"
	}
	nc, err = nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Unable to connect to NATS: %v\n", err)
	}
}

func main() {
	initConnections()
	defer db.Close()
	defer nc.Close()

	_, err := nc.Subscribe("job.created", func(m *nats.Msg) {
		jobID, err := strconv.Atoi(string(m.Data))
		if err != nil {
			log.Printf("Invalid job ID: %v", string(m.Data))
			return
		}

		log.Printf("Worker received job %d", jobID)
		
		// Update status to PROCESSING
		_, err = db.Exec(context.Background(), "UPDATE jobs SET status = 'PROCESSING', updated_at = NOW() WHERE id = $1", jobID)
		if err != nil {
			log.Printf("Failed to update job %d to PROCESSING: %v", jobID, err)
			return
		}

		// Simulate work
		time.Sleep(3 * time.Second)

		// Update status to COMPLETED
		_, err = db.Exec(context.Background(), "UPDATE jobs SET status = 'COMPLETED', updated_at = NOW() WHERE id = $1", jobID)
		if err != nil {
			log.Printf("Failed to update job %d to COMPLETED: %v", jobID, err)
			return
		}

		log.Printf("Worker completed job %d", jobID)
		nc.Publish("job.completed", []byte(strconv.Itoa(jobID)))
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("Worker is listening for jobs...")

	// Wait for shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Worker shutting down")
}
