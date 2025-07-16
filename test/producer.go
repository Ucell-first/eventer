package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/IBM/sarama"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    uint16    `json:"status"`
	LatencyMs uint32    `json:"latency_ms"`
	IP        net.IP    `json:"ip"`
}

func main() {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	paths := []string{"/api/users", "/api/orders", "/api/products", "/health"}
	statuses := []uint16{200, 201, 400, 404, 500}
	ips := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"}

	log.Println("Starting to send test logs...")

	for i := 0; i < 1000; i++ {
		logEntry := LogEntry{
			Timestamp: time.Now(),
			Method:    methods[rand.Intn(len(methods))],
			Path:      paths[rand.Intn(len(paths))],
			Status:    statuses[rand.Intn(len(statuses))],
			LatencyMs: uint32(rand.Intn(1000)),
			IP:        net.ParseIP(ips[rand.Intn(len(ips))]),
		}

		data, _ := json.Marshal(logEntry)

		msg := &sarama.ProducerMessage{
			Topic: "logs",
			Value: sarama.StringEncoder(data),
		}

		_, _, err := producer.SendMessage(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
		} else {
			log.Printf("Sent log: %s %s %d", logEntry.Method, logEntry.Path, logEntry.Status)
		}

		time.Sleep(100 * time.Millisecond)
	}

	log.Println("Finished sending test logs")
}
