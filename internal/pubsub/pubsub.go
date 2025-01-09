package pubsub

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int
type AckType int

const (
	Durable SimpleQueueType = iota
	Transient
)

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		log.Println("Failed to marshal JSON:", err)
		return err
	}

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        bytes,
		},
	)
	if err != nil {
		log.Printf("Failed to publish message to %s@%s: %v\n", exchange, key, err)
		return err
	}

	return nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
	)
	if err != nil {
		log.Printf(
			"Failed to declare and bind to %s@%s: %v\n",
			exchange,
			key,
			err,
		)
		return err
	}

	deliveryChan, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("Failed to consume messages from queue %s: %v\n", queue.Name, err)
		return err
	}

	go func() {
		for delivery := range deliveryChan {
			var val T
			err := json.Unmarshal(delivery.Body, &val)
			if err != nil {
				log.Printf("Failed to unmarshal JSON: %v\n", err)
				continue
			}

			ackType := handler(val)

			switch ackType {
			case Ack:
				err = delivery.Ack(false)
				log.Println("Acked message")
			case NackRequeue:
				err = delivery.Nack(false, true)
				log.Println("Nacked and requeued message")
			case NackDiscard:
				err = delivery.Nack(false, false)
				log.Println("Nacked and discarded message")
			}

			if err != nil {
				log.Printf("Failed to ack message: %v\n", err)
			}
		}
	}()

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		log.Println("Failed to open a channel:", err)
		return nil, amqp.Queue{}, err
	}

	var durable, autoDelete, exclusive bool
	if simpleQueueType == Durable {
		durable = true
		autoDelete = false
		exclusive = false
	} else if simpleQueueType == Transient {
		durable = false
		autoDelete = true
		exclusive = true
	}

	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, nil)
	if err != nil {
		log.Printf("Failed to declare queue %s: %v\n", queueName, err)
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		log.Printf("Failed to bind queue %s to %s@%s: %v\n", queue.Name, exchange, key, err)
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}
