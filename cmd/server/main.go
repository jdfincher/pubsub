package main

import (
	"fmt"
	"os"
	"os/signal"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	rabbitURI := "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(rabbitURI)
	if err != nil {
		fmt.Println("Something went wrong establishing amqp connection", err)
		os.Exit(1)
	}
	defer connection.Close()
	fmt.Println("Rabbitmq connection was successfull...")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("\nProgram terminating...")
	os.Exit(0)
}
