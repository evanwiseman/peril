package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func cleanup() {
	log.Print("Stopping Peril server...")
}

func main() {
	// Capture ctrl + ctrlC for cleanup
	ctrlC := make(chan os.Signal, 1.)
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctrlC
		cleanup()
		os.Exit(1)
	}()

	// Start up the server
	log.Println("Starting Peril server...")
	gamelogic.PrintServerHelp()

	// Start Rabbit MQ using the rabbitMQUrl
	const rabbitMQUrl = "amqp://guest:guest@localhost:5672/"
	rabbitMQConnection, err := amqp.Dial(rabbitMQUrl)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ server: %v\n", err)
	}
	defer rabbitMQConnection.Close()
	log.Printf("Successfully connected to Rabbit MQ Server.\n")

	// Open a Rabbit MQ channel
	rabbitMQChannel, err := rabbitMQConnection.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v\n", err)
	}
	defer rabbitMQChannel.Close()

	_, _, err = pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		fmt.Sprintf("%v.*", routing.GameLogSlug),
		"durable",
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind: %v\n", err)
	}

	// Start the REPL
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "pause":
			log.Println("Sending pause message.")
			err = pubsub.PublishJSON(
				rabbitMQChannel,
				routing.ExchangePerilDirect,
				routing.PauseKey,
				routing.PlayingState{
					IsPaused: true,
				},
			)
			if err != nil {
				log.Printf("Failed to publish pause message: %v\n", err)
			}
		case "resume":
			log.Println("Sending resume message.")
			err = pubsub.PublishJSON(
				rabbitMQChannel,
				routing.ExchangePerilDirect,
				routing.PauseKey,
				routing.PlayingState{
					IsPaused: false,
				},
			)
			if err != nil {
				log.Printf("Failed to publish resume message: %v\n", err)
			}
		case "quit":
			cleanup()
			os.Exit(1)
		default:
			log.Printf("Invalid command: %v.", words[0])
		}
	}

}
