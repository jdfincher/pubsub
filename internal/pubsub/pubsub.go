// Package pubsub implements shared code between server and client
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType struct {
	Durable    bool
	AutoDelete bool
	Exclusive  bool
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		fmt.Println("Error marshalling data ->", err)
		return err
	}

	if err := ch.PublishWithContext(
		context.Background(),
		exchange, key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		}); err != nil {
		fmt.Println("Error publishing message to exchange ->", err)
		return err
	}
	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()
	if err != nil {
		fmt.Println("Error creating amqp channel ->", err)
		os.Exit(1)
	}

	queue, err := channel.QueueDeclare(
		queueName,
		queueType.Durable,
		queueType.AutoDelete,
		queueType.Exclusive,
		false,
		nil)
	if err != nil {
		fmt.Println("Error declaring queue ->", err)
		os.Exit(1)
	}

	if err = channel.QueueBind(queueName, key, exchange, false, nil); err != nil {
		fmt.Println("Error binding queue ->", err)
		os.Exit(1)
	}

	return channel, queue, nil
}
