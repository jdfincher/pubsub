// Package pubsub implements shared code between server and client
package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
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

type AckType struct {
	Ack         bool
	NackRequeue bool
	NackDiscard bool
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

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		fmt.Println("Error declaring and binding channel and queue ->", err)
		return err
	}
	deliveryChan, err := ch.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		fmt.Println("Error creating delivery channel", err)
		return err
	}
	go func() {
		for msg := range deliveryChan {
			var delivery T
			err := json.Unmarshal(msg.Body, &delivery)
			if err != nil {
				fmt.Println("Error unmarshalling msg body", err)
			}
			ack := handler(delivery)
			if ack.Ack {
				err = msg.Ack(false)
				if err != nil {
					fmt.Println("Error sending Ack", err)
				} else {
					fmt.Println("Successfully sent Ack")
				}
			} else if ack.NackRequeue {
				err = msg.Nack(false, true)
				if err != nil {
					fmt.Println("Error sending Nack Requeue", err)
				} else {
					fmt.Println("Successfully sent Nack Requeue")
				}
			} else if ack.NackDiscard {
				err = msg.Nack(false, false)
				if err != nil {
					fmt.Println("Error sending Nack Discard", err)
				} else {
					fmt.Println("Successfully sent Nack Discard")
				}
			}
		}
	}()
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

	deadLetter := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}

	queue, err := channel.QueueDeclare(
		queueName,
		queueType.Durable,
		queueType.AutoDelete,
		queueType.Exclusive,
		false,
		deadLetter)
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

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	buf := bytes.Buffer{}
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(val); err != nil {
		fmt.Println("Error encoding to gob ->", err)
		os.Exit(1)
	}
	if err := ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        buf.Bytes(),
		}); err != nil {
		fmt.Println("Error publishing gob msg to exchange ->", err)
		return err
	}
	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) AckType,
) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		fmt.Println("Error declaring and binding channel and queue ->", err)
	}
	err = ch.Qos(10, 0, false)
	if err != nil {
		fmt.Println("Error setting prefetch size")
	}

	deliveryChan, err := ch.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		fmt.Println("Error creating delivery channel", err)
		return err
	}
	go func() {
		for msg := range deliveryChan {
			var delivery T
			buf := bytes.NewBuffer(msg.Body)
			decoder := gob.NewDecoder(buf)
			if err := decoder.Decode(&delivery); err != nil {
				fmt.Println("Error decoding delivered gob to gamelog struct", err)
			}
			ack := handler(delivery)
			if ack.Ack {
				err = msg.Ack(false)
				if err != nil {
					fmt.Println("Error sending Ack", err)
				} else {
					fmt.Println("Successfully sent Ack")
				}
			} else if ack.NackRequeue {
				err = msg.Nack(false, true)
				if err != nil {
					fmt.Println("Error sending NackRequeue", err)
				} else {
					fmt.Println("Successfully sent NackRequeue")
				}
			} else if ack.NackDiscard {
				err = msg.Nack(false, false)
				if err != nil {
					fmt.Println("Error sending Nack Discard", err)
				} else {
					fmt.Println("Successfully sent Nack Discard")
				}
			}
		}
	}()
	return nil
}
