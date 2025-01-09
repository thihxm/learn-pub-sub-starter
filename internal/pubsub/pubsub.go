package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
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

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
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
			val, err := unmarshaller(delivery.Body)
			if err != nil {
				log.Printf("Failed to unmarshal body: %v\n", err)
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
			fmt.Print("> ")
		}
	}()

	return nil
}

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
	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		func(body []byte) (T, error) {
			var val T
			if err := json.Unmarshal(body, &val); err != nil {
				log.Printf("Failed to unmarshal JSON: %v\n", err)
				return val, err
			}
			return val, nil
		},
	)
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(val); err != nil {
		log.Println("Failed to encode gob:", err)
		return err
	}

	err := ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        buf.Bytes(),
		},
	)
	if err != nil {
		log.Printf("Failed to publish message to %s@%s: %v\n", exchange, key, err)
		return err
	}

	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
) error {
	return subscribe(
		conn,
		exchange,
		queueName,
		key,
		simpleQueueType,
		handler,
		func(body []byte) (T, error) {
			buf := bytes.NewBuffer(body)
			dec := gob.NewDecoder(buf)
			var val T
			if err := dec.Decode(&val); err != nil {
				log.Printf("Failed to decode gob: %v\n", err)
				return val, err
			}
			return val, nil
		},
	)
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

	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	})
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
