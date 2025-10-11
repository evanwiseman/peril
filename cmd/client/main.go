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

func main() {
	fmt.Println("Starting Peril client...")

	const url = "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("Failed to connect to to RabbitMQ server: %v\n", err)
	}
	defer conn.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("Failed to get username: %v\n", err)
	}

	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		fmt.Sprintf("%v.%v", routing.PauseKey, username),
		routing.PauseKey,
		"transient",
	)
	if err != nil {
		log.Fatalf("Failed to declare and bind: %v\n", err)
	}

	// Capture ctrl + c for cleanup
	c := make(chan os.Signal, 1.)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Print("Stopping Peril client...")
	os.Exit(1)
}
