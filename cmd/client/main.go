package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	amqpURI = "amqp://guest:guest@localhost:5672/"
)

type apiConfig struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	username string
}

func main() {
	log.Println("Starting Peril client...")

	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		log.Fatalln("Failed to connect to RabbitMQ:", err)
		return
	}
	cfg := &apiConfig{
		conn: conn,
	}
	defer cfg.conn.Close()

	ch, err := cfg.conn.Channel()
	if err != nil {
		log.Fatalln("Failed to open a channel:", err)
		return
	}
	cfg.ch = ch
	defer cfg.ch.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalln("Failed to get username:", err)
		return
	}
	cfg.username = username

	gameState := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		cfg.conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%s.%s", routing.PauseKey, username),
		routing.PauseKey,
		pubsub.Transient,
		cfg.handlerPause(gameState),
	)
	if err != nil {
		log.Fatalln("Failed to subscribe to Pause messages:", err)
		return
	}

	err = pubsub.SubscribeJSON(
		cfg.conn,
		routing.ExchangePerilTopic,
		fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username),
		fmt.Sprintf("%s.*", routing.ArmyMovesPrefix),
		pubsub.Transient,
		cfg.handlerMove(gameState),
	)
	if err != nil {
		log.Fatalln("Failed to subscribe to Move messages:", err)
		return
	}

	err = pubsub.SubscribeJSON(
		cfg.conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix),
		pubsub.Durable,
		cfg.handlerWar(gameState),
	)
	if err != nil {
		log.Fatalln("Failed to subscribe to War messages:", err)
		return
	}

	for {
		input := gamelogic.GetInput()

		if len(input) == 0 {
			continue
		}

		cmd := input[0]

		switch cmd {
		case "spawn":
			err := gameState.CommandSpawn(input)
			if err != nil {
				fmt.Println(err)
			}
		case "move":
			move, err := gameState.CommandMove(input)
			if err != nil {
				fmt.Println(err)
			}
			err = pubsub.PublishJSON(
				cfg.ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username),
				move,
			)
			if err != nil {
				fmt.Println("Failed to publish move:", err)
			}
			fmt.Printf("Published move of %d unit(s) to %s\n", len(move.Units), move.ToLocation)
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		}
	}
}

func (cfg *apiConfig) handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)

		return pubsub.Ack
	}
}

func (cfg *apiConfig) handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)

		if outcome == gamelogic.MoveOutcomeMakeWar {
			err := pubsub.PublishJSON(
				cfg.ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, move.Player.Username),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.Player,
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		}

		if outcome == gamelogic.MoveOutComeSafe {
			return pubsub.Ack
		}

		return pubsub.NackDiscard
	}
}

func (cfg *apiConfig) handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(recognition gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(recognition)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon, gamelogic.WarOutcomeDraw:
			message := fmt.Sprintf("%s won a war against %s", winner, loser)
			if outcome == gamelogic.WarOutcomeDraw {
				message = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			}

			err := pubsub.PublishGob(
				cfg.ch,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%s.%s", routing.GameLogSlug, recognition.Attacker.Username),
				routing.GameLog{
					CurrentTime: time.Now(),
					Message:     message,
					Username:    gs.GetUsername(),
				},
			)
			if err != nil {
				log.Println("Failed to publish game log:", err)
				return pubsub.NackRequeue
			}
			fmt.Println("Published game log")
			return pubsub.Ack
		default:
			fmt.Printf("Error: Unexpected war outcome -> %d", outcome)
			return pubsub.NackDiscard
		}
	}
}
