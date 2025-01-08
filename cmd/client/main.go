package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	amqpURI = "amqp://guest:guest@localhost:5672/"
)

func main() {
	log.Println("Starting Peril client...")

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		log.Fatalln("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalln("Failed to get username:", err)
		return
	}

	ch, queue, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, fmt.Sprintf("%s.%s", routing.PauseKey, username), routing.PauseKey, pubsub.Transient)
	if err != nil {
		log.Fatalln("Failed to declare and bind queue to exchange:", err)
		return
	}
	messagesChan, err := ch.Consume(queue.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalln("Failed to consume messages from queue:", err)
		return
	}

	message := <-messagesChan
	log.Printf("Received message: %s\n", message.Body)
}
