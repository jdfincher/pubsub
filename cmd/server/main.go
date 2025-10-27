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

	err = pubsub.SubscribeGob(
		connection,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+"."+"*",
		pubsub.SimpleQueueType{
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
		},
		handlerGameLog,
	)
	if err != nil {
		fmt.Println("Error subscribing to gamelogs queue ->", err)
	}

	gamelogic.PrintServerHelp()
	for {
		userInput := gamelogic.GetInput()
		if len(userInput) == 0 {
			continue
		} else if userInput[0] == "pause" {
			fmt.Println("Sending pause message")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				fmt.Println("Error calling PublishJSON for pause message ->", err)
				os.Exit(1)
			} else {
				continue
			}
		} else if userInput[0] == "resume" {
			fmt.Println("Sending resume message")
			err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				fmt.Println("Error calling PublishJSON for resume message ->", err)
				os.Exit(1)
			} else {
				continue
			}
		} else if userInput[0] == "quit" {
			fmt.Println("Shutting down server, goodbye...")
			os.Exit(0)
		}
		fmt.Println("Unknown command, ignoring and continuing...")

	}
}

func handlerGameLog(gl routing.GameLog) pubsub.AckType {
	if err := gamelogic.WriteLog(gl); err != nil {
		return pubsub.AckType{NackRequeue: true}
	}
	return pubsub.AckType{Ack: true}
}
