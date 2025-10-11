package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	// Marshal val to JSON []byte
	var data []byte
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marshal val: %v", err)
	}

	// Publish the message to the exchange
	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message to exchange: %v", err)
	}

	return nil
}
