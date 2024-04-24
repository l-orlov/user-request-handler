package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Shopify/sarama"
)

const (
	brokerURL = "localhost:9092"
	topicName = "messages"
)

// Message struct
type Message struct {
	Text string `form:"text" json:"text"`
}

func main() {
	// Connect to kafka
	brokersUrl := []string{brokerURL}
	producer, err := ConnectProducer(brokersUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	i := 0
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ticker.C:
			// Create msg
			msg := &Message{
				Text: fmt.Sprintf("Message with number %d", i),
			}
			msgInBytes, err := json.Marshal(msg)
			if err != nil {
				log.Fatal(err)
			}

			// Send msg to kafka
			err = PushMsgToQueue(producer, topicName, msgInBytes)
			if err != nil {
				log.Fatal(err)
			}

			i++
		}
	}
}

func PushMsgToQueue(producer sarama.SyncProducer, topic string, message []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(message),
	}

	partition, offset, err := producer.SendMessage(msg)
	if err != nil {
		return err
	}

	fmt.Printf("Message is stored in topic(%s)/partition(%d)/offset(%d)\n", topic, partition, offset)

	return nil
}

func ConnectProducer(brokersUrl []string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	conn, err := sarama.NewSyncProducer(brokersUrl, config)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
