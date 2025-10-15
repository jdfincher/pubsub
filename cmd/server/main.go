package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")

	connection, err := amqp.Dial(routing.RabbitURI)
	if err != nil {
		fmt.Println("Error establishing amqp connection ->", err)
		os.Exit(1)
	}
	defer connection.Close()
	fmt.Println("Rabbitmq connection was successfull...")

	channel, err := connection.Channel()
	if err != nil {
		fmt.Println("Error creating amqp channel ->", err)
		os.Exit(1)
	}

	err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
	if err != nil {
		fmt.Println("Error calling PublishJSON ->", err)
		os.Exit(1)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("\nProgram terminating...")
	os.Exit(0)
}
