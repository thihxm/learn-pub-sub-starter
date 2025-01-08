package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
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

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		sig := <-signalChan
		log.Printf("Received signal: %v\n", sig)
		log.Println("Closing connection...")
		conn.Close()
		log.Println("Connection closed!")
		os.Exit(0)
	}()

	gamelogic.PrintServerHelp()

	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		fmt.Sprintf("%s.*", routing.GameLogSlug),
		pubsub.Durable,
	)
	if err != nil {
		log.Fatalf(
			"Failed to declare and bind to %s@%s: %v\n",
			routing.ExchangePerilTopic,
			routing.GameLogSlug,
			err,
		)
		return
	}

	for {
		input := gamelogic.GetInput()

		if len(input) == 0 {
			continue
		}

		cmd := input[0]

		switch cmd {
		case "pause":
			log.Println("Pausing game...")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: true,
			})
			if err != nil {
				log.Fatalln("Failed to publish message:", err)
				return
			}
			log.Println("Game paused!")
		case "resume":
			log.Println("Resuming game...")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: false,
			})
			if err != nil {
				log.Fatalln("Failed to publish message:", err)
				return
			}
			log.Println("Game resumed!")
		case "quit":
			log.Println("Quitting game...")
			return
		case "help":
			gamelogic.PrintServerHelp()
		default:
			log.Println("Unknown command. Please try again.")
			gamelogic.PrintServerHelp()
		}
	}
}
