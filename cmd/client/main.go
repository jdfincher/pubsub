package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	fmt.Println("Rabbitmq connection was sucessfull...")

	channel, err := connection.Channel()
	if err != nil {
		fmt.Println("Error creating amqp channel ->", err)
		os.Exit(1)
	}

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	pauseQueue := strings.Join([]string{routing.PauseKey, userName}, ".")
	armyMovesQueue := strings.Join([]string{routing.ArmyMovesPrefix, userName}, ".")
	warQueue := strings.Join([]string{routing.WarRecognitionsPrefix, userName}, ".")

	gameState := gamelogic.NewGameState(userName)

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		pauseQueue,
		routing.PauseKey,
		pubsub.SimpleQueueType{
			Durable:    false,
			AutoDelete: true,
			Exclusive:  true,
		},
		handlerPause(gameState),
	)
	if err != nil {
		fmt.Println("Error subscribing to pause queue", err)
		os.Exit(1)
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		armyMovesQueue,
		"army_moves.*",
		pubsub.SimpleQueueType{
			Durable:    false,
			AutoDelete: true,
			Exclusive:  true,
		},
		handlerMove(gameState, channel, warQueue),
	)
	if err != nil {
		fmt.Println("Error subscribing to moves queue", err)
		os.Exit(1)
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		warQueue,
		pubsub.SimpleQueueType{
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
		},
		handlerWar(gameState, channel, routing.ExchangePerilTopic),
	)
	if err != nil {
		fmt.Println("Error subscribing to war queue", err)
		os.Exit(1)
	}

	for {
		userInput := gamelogic.GetInput()
		if len(userInput) == 0 {
			continue
		} else if userInput[0] == "spawn" {
			if err := gameState.CommandSpawn(userInput); err != nil {
				fmt.Println(err)
				continue
			}
		} else if userInput[0] == "move" {
			armyMove, err := gameState.CommandMove(userInput)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = pubsub.PublishJSON(channel, routing.ExchangePerilTopic, armyMovesQueue, armyMove)
			if err != nil {
				fmt.Println("Error publishing move to exchange ->", err)
				continue
			}
			fmt.Println("Move was successful", armyMove)
			continue
		} else if userInput[0] == "status" {
			gameState.CommandStatus()
			continue
		} else if userInput[0] == "help" {
			gamelogic.PrintClientHelp()
			continue
		} else if userInput[0] == "spam" {
			if len(userInput) < 2 {
				fmt.Println("Must provide number of times to spam like 'spam 100'")
				continue
			} else {
				n, err := strconv.Atoi(userInput[1])
				if err != nil {
					fmt.Println("Must provide whole integer as spam count")
					continue
				}
				err = spamDeploy(channel, routing.ExchangePerilTopic, userName, n)
				if err != nil {
					fmt.Println(err)
					continue
				}
			}
			continue
		} else if userInput[0] == "quit" {
			gamelogic.PrintQuit()
			os.Exit(0)
		} else {
			fmt.Println("Unrecognized command used")
			continue
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.AckType{Ack: true}
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel, war string) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.AckType{NackDiscard: true}

		case gamelogic.MoveOutcomeSafe:
			return pubsub.AckType{Ack: true}

		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(ch,
				routing.ExchangePerilTopic,
				war,
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.GetPlayerSnap(),
				})
			if err != nil {
				fmt.Println("Error publishing war message ->, Nack Requeueing the msg", err)
				return pubsub.AckType{NackRequeue: true}
			}
			return pubsub.AckType{Ack: true}
		}
		return pubsub.AckType{Ack: true}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel, exchange string) func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
	defer fmt.Print("> ")
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		outcome, winner, loser := gs.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.AckType{NackRequeue: true}

		case gamelogic.WarOutcomeNoUnits:
			return pubsub.AckType{NackDiscard: true}

		case gamelogic.WarOutcomeOpponentWon:
			warLog := fmt.Sprintf("%s won a war against %s\n", winner, loser)
			return publishGameLog(ch, exchange, rw.Attacker.Username, warLog)

		case gamelogic.WarOutcomeYouWon:
			warLog := fmt.Sprintf("%s won a war against %s\n", winner, loser)
			return publishGameLog(ch, exchange, rw.Attacker.Username, warLog)

		case gamelogic.WarOutcomeDraw:
			warLog := fmt.Sprintf("A war between %s and %s resulted in a draw\n", winner, loser)
			return publishGameLog(ch, exchange, rw.Attacker.Username, warLog)

		default:
			fmt.Println("Error in outcome of war, unrecognized outcome. Good luck chuck!")
			return pubsub.AckType{NackDiscard: true}
		}
	}
}

func publishGameLog(ch *amqp.Channel, exchange, userName string, warLog string) pubsub.AckType {
	gameLog := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     warLog,
		Username:    userName,
	}
	if err := pubsub.PublishGob(ch, exchange, routing.GameLogSlug+"."+userName, gameLog); err != nil {
		fmt.Println("error: publishing gamelog failed ->", err)
		return pubsub.AckType{NackRequeue: true}
	}
	return pubsub.AckType{Ack: true}
}

func spamDeploy(ch *amqp.Channel, exchange, userName string, n int) error {
	for range n {
		log := routing.GameLog{
			CurrentTime: time.Now(),
			Message:     gamelogic.GetMaliciousLog(),
			Username:    userName,
		}
		err := pubsub.PublishGob(
			ch,
			exchange,
			routing.GameLogSlug+"."+userName,
			log,
		)
		if err != nil {
			return fmt.Errorf("error: issue publishing spam msg %w", err)
		}
	}
	return nil
}
