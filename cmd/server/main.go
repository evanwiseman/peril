package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	log.Println("Starting Peril server...")

	// Start Rabbit MQ using the address
	const rmqAddr = "amqp://guest:guest@localhost:5672/"
	rmqConnection, err := amqp.Dial(rmqAddr)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ server.\n")
	}
	defer rmqConnection.Close()
	log.Printf("Successfully connected to Rabbit MQ Server.\n")

	// Open a Rabbit MQ channel
	rmqChannel, err := rmqConnection.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel.\n")
	}
	defer rmqChannel.Close()

	// Publish a JSON message to the exchange
	pubsub.PublishJSON(
		rmqChannel,
		routing.ExchangePerilDirect,
		routing.PauseKey,
		routing.PlayingState{
			IsPaused: true,
		},
	)

	// Capture ctrl + c for cleanup
	c := make(chan os.Signal, 1.)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Print("Stopping Peril server...")
	os.Exit(1)
}
