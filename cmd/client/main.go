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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) routing.AckType {
	return func(state routing.PlayingState) routing.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(state)
		return routing.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) routing.AckType {
	return func(move gamelogic.ArmyMove) routing.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe, gamelogic.MoveOutcomeMakeWar:
			return routing.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return routing.NackDiscard
		default:
			return routing.NackDiscard
		}
	}
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

	// Declare pause queue and routing name/key
	pauseQueueName := fmt.Sprintf("%v.%v", routing.PauseKey, username)
	pauseRoutingKey := routing.PauseKey
	pauseQueueType := "transient"

	// Bind the pause exchange
	_, _, err = pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilDirect,
		pauseQueueName,
		pauseRoutingKey,
		pauseQueueType,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind pause exchange: %v\n", err)
	}

	gs := gamelogic.NewGameState(username)

	// Subscribe to pause/resume
	err = pubsub.SubscribeJSON(
		rabbitMQConnection,
		routing.ExchangePerilDirect,
		pauseQueueName,
		pauseRoutingKey,
		pauseQueueType,
		handlerPause(gs),
	)
	if err != nil {
		log.Fatalf("Failed to subscribe Pause/Resume JSON: %v", err)
	}

	// Declare move queue and routing name/key
	movesQueueName := fmt.Sprintf("%v.%v", routing.ArmyMovesPrefix, username)
	movesRoutingKey := fmt.Sprintf("%v.*", routing.ArmyMovesPrefix)
	movesQueueType := "transient"

	// Bind the move exchange
	movesChannel, _, err := pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		movesQueueName,
		movesRoutingKey,
		movesQueueType,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind move exchange: %v\n", err)
	}

	// Subscribe to moves
	err = pubsub.SubscribeJSON(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		movesQueueName,
		movesRoutingKey,
		movesQueueType,
		handlerMove(gs),
	)
	if err != nil {
		log.Fatalf("Faield to subscribe moves JSON: %v", err)
	}

	// Start the REPL
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
			move, err := gs.CommandMove(words)

			if err != nil {
				log.Printf("Failed to move unit: %v\n", err)
				continue
			}

			err = pubsub.PublishJSON(
				movesChannel,
				routing.ExchangePerilTopic,
				movesQueueName,
				move,
			)
			if err != nil {
				log.Printf("Failed to publish move JSON: %v\n", err)
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
