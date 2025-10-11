package pubsub

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType string, // an enum to represent "durable" or "transient"
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
