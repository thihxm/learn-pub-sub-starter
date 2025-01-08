package main

import (
	"log"
	"os"
	"os/signal"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	amqpURI = "amqp://guest:guest@localhost:5672/"
)

func main() {
	log.Println("Starting Peril server...")

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		log.Fatalln("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()
	log.Println("Connected to RabbitMQ!")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	sig := <-signalChan
	log.Printf("Received signal: %v\n", sig)
	log.Println("Closing connection...")
	conn.Close()
	log.Println("Connection closed!")
	os.Exit(0)
}
