package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	// Marshal val to JSON []byte
	var data []byte
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed JSON marshal val: %v", err)
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
		return fmt.Errorf("failed publish JSON message to exchange: %v", err)
	}

	return nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	// Encode to gob
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(val)
	if err != nil {
		return fmt.Errorf("failed gob encode val: %v", err)
	}

	data := buf.Bytes()

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        data,
		},
	)
	if err != nil {
		return fmt.Errorf("failed publish gob message to exchange: %v", err)
	}

	return nil
}
