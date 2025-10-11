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
	log.Print("Stopping Peril client...")
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

	// Star the client
	fmt.Println("Starting Peril client...")

	// Start Rabbit MQ using the rabbitMQUrl
	const rabbitMQUrl = "amqp://guest:guest@localhost:5672/"
	rabbitMQConnection, err := amqp.Dial(rabbitMQUrl)
	if err != nil {
		log.Fatalf("Failed to connect to to RabbitMQ server: %v\n", err)
	}
	defer rabbitMQConnection.Close()

	// Get the username provided by user
	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Failed to get username: %v\n", err)
	}

	// Bind the pause exchange
	_, _, err = pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%v.%v", routing.PauseKey, username),
		routing.PauseKey,
		"transient",
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind: %v\n", err)
	}

	// Start the REPL
	gs := gamelogic.NewGameState(username)
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "spawn":
			err = gs.CommandSpawn(words)
			if err != nil {
				log.Printf("Failed to spawn unit: %v\n", err)
			}
		case "move":
			_, err := gs.CommandMove(words)
			if err != nil {
				log.Printf("Failed to move unit: %v\n", err)
			} else {
				log.Printf("Move unit %v to %v successful\n", words[2], words[1])
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			log.Printf("Spamming not allowed yet!")
		case "quit":
			cleanup()
			os.Exit(1)
		default:
			log.Printf("Invalid command: %v.", words[0])
			continue
		}
	}
}
