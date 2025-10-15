package main

import (
	"fmt"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")

	connection, err := amqp.Dial(routing.RabbitURI)
	if err != nil {
		fmt.Println("Error establishing amqp connection ->", err)
		os.Exit(1)
	}
	defer connection.Close()

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, _, err = pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilDirect,
		"pause"+"."+userName,
		routing.PauseKey,
		pubsub.SimpleQueueType{
			Durable:    false,
			AutoDelete: true,
			Exclusive:  true,
		})
}
