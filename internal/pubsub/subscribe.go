package pubsub

import (
	"encoding/json"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType string, // represents "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	// Create a new ch
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	// Declare a new queue
	var isDurable bool
	var isAutoDelete bool
	var isExclusive bool
	switch queueType {
	case "durable":
		isDurable = true
		isAutoDelete = false
		isExclusive = false
	case "transient":
		isDurable = false
		isAutoDelete = true
		isExclusive = true
	}
	q, err := ch.QueueDeclare(
		queueName,
		isDurable,
		isAutoDelete,
		isExclusive,
		false,
		nil,
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	// Bind the queue to the exchange
	err = ch.QueueBind(queueName, key, exchange, false, amqp.Table{})
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	// Return the channel and the queue
	return ch, q, nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType string, // represents "durable" or "transient"
	handler func(T) routing.AckType,
) error {
	// Mak sure the queue exists
	ch, _, err := DeclareAndBind(
		conn,
		exchange,
		queueName,
		key,
		queueType,
	)
	if err != nil {
		return nil
	}

	// Get a chan of deliveries by consuming message
	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		amqp.Table{},
	)
	if err != nil {
		return nil
	}

	go func() {
		for msg := range msgs {
			// Unmarshal deliveries
			var obj T
			err = json.Unmarshal(msg.Body, &obj)
			if err != nil {
				return
			}

			// Send the object of T to the handler
			ackType := handler(obj)
			switch ackType {
			case routing.Ack:
				log.Println("Sending Ack")
				msg.Ack(false)
			case routing.NackRequeue:
				log.Println("Sending Nack Requeue")
				msg.Nack(false, true)
			case routing.NackDiscard:
				log.Println("Sending Nack Discard")
				msg.Nack(false, false)
			}
		}
	}()

	return nil
}
