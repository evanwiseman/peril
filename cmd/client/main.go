package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

func handlerMove(gs *gamelogic.GameState, warCh *amqp.Channel) func(gamelogic.ArmyMove) routing.AckType {
	return func(move gamelogic.ArmyMove) routing.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return routing.Ack
		case gamelogic.MoveOutcomeMakeWar:
			// Publish message
			err := pubsub.PublishJSON(
				warCh,
				routing.ExchangePerilTopic,
				fmt.Sprintf("%v.%v", routing.WarRecognitionsPrefix, gs.Player.Username),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				return routing.NackRequeue
			}
			return routing.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return routing.NackDiscard
		default:
			return routing.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, glCh *amqp.Channel) func(gamelogic.RecognitionOfWar) routing.AckType {
	return func(rw gamelogic.RecognitionOfWar) routing.AckType {
		defer fmt.Print("> ")

		outcome, winner, loser := gs.HandleWar(rw)
		gl := routing.GameLog{
			CurrentTime: time.Now(),
			Username:    gs.GetUsername(),
		}

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return routing.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return routing.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon:
			gl.Message = fmt.Sprintf("%v won a war against %v", winner, loser)
			err := publishGameLog(glCh, gl)
			if err != nil {
				return routing.NackRequeue
			}
			return routing.Ack
		case gamelogic.WarOutcomeDraw:
			gl.Message = fmt.Sprintf("A war between %v and %v, resulted in a draw", winner, loser)
			err := publishGameLog(glCh, gl)
			if err != nil {
				return routing.NackRequeue
			}
			return routing.Ack
		default:
			log.Println("Invalid war outcome")
			return routing.NackDiscard
		}
	}
}

func publishGameLog(ch *amqp.Channel, gl routing.GameLog) error {
	routingKey := fmt.Sprintf("%v.%v", routing.GameLogSlug, gl.Username)
	err := pubsub.PublishGob(
		ch,
		string(routing.ExchangePerilTopic),
		routingKey,
		gl,
	)
	return err
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

	/**************************************************************************
	RabbitMQ
	**************************************************************************/
	// Start the client
	fmt.Println("Starting Peril client...")

	// Start Rabbit MQ using the rabbitMQUrl
	const rabbitMQUrl = "amqp://guest:guest@localhost:5672/"
	rabbitMQConnection, err := amqp.Dial(rabbitMQUrl)
	if err != nil {
		log.Fatalf("Failed to connect to to RabbitMQ server: %v\n", err)
	}
	defer rabbitMQConnection.Close()

	/**************************************************************************
	GameState
	**************************************************************************/
	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Failed to get username: %v\n", err)
	}

	// Create the game state
	gs := gamelogic.NewGameState(username)

	/**************************************************************************
	GameLogs
	**************************************************************************/
	glQueueName := routing.GameLogSlug
	glRoutingKey := routing.GameLogSlug
	glQueueType := routing.Durable

	glChannel, _, err := pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		glQueueName,
		glRoutingKey,
		glQueueType,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind game log exchange: %v\n", err)
	}

	/**************************************************************************
	RabbitMQ Pause
	**************************************************************************/
	pauseQueueName := fmt.Sprintf("%v.%v", routing.PauseKey, username)
	pauseRoutingKey := routing.PauseKey
	pauseQueueType := routing.Transient

	// Create transient exchange per user/clien
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

	/**************************************************************************
	RabbitMQ War
	**************************************************************************/
	warQueueName := routing.WarRecognitionsPrefix                       // durable shared queue
	warRoutingKey := fmt.Sprintf("%v.*", routing.WarRecognitionsPrefix) // match all usernames
	warQueueType := routing.Durable

	// Create durable shared exchange
	warChannel, _, err := pubsub.DeclareAndBind(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		warQueueName,
		warRoutingKey,
		warQueueType,
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind war queue: %v", err)
	}

	// Subscribe to war
	err = pubsub.SubscribeJSON(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		warQueueName,
		warRoutingKey,
		warQueueType,
		handlerWar(gs, glChannel),
	)
	if err != nil {
		log.Fatalf("Failed to subscribe to war JSON: %v", err)
	}

	/**************************************************************************
	RabbitMQ Moves
	**************************************************************************/
	movesQueueName := fmt.Sprintf("%v.%v", routing.ArmyMovesPrefix, username)
	movesRoutingKey := fmt.Sprintf("%v.*", routing.ArmyMovesPrefix)
	movesQueueType := routing.Transient

	// Create transient exchange per user/client
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

	// Subscribe to all moves
	err = pubsub.SubscribeJSON(
		rabbitMQConnection,
		routing.ExchangePerilTopic,
		movesQueueName,
		movesRoutingKey,
		movesQueueType,
		handlerMove(gs, warChannel),
	)
	if err != nil {
		log.Fatalf("Failed to subscribe moves JSON: %v", err)
	}

	/**************************************************************************
	REPL
	**************************************************************************/
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
