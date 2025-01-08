package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
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

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalln("Failed to open a channel:", err)
		return
	}

	err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
		IsPaused: true,
	})
	if err != nil {
		log.Fatalln("Failed to publish message:", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	sig := <-signalChan
	log.Printf("Received signal: %v\n", sig)
	log.Println("Closing connection...")
	conn.Close()
	log.Println("Connection closed!")
	os.Exit(0)
}
