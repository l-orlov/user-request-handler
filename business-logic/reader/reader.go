package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Shopify/sarama"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Custom metric of type counter
var messageResult = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "message_handling_result_count",
		Help: "Count of message handling result",
	},
	[]string{"result"},
)

func init() {
	// Register the counter so prometheus can collect this metric
	prometheus.MustRegister(messageResult)
}

const (
	envBrokerURLName = "BROKER_URL"
	envDBURIName     = "DB_URI"
)

var (
	// Broker config
	brokerURL       = "localhost:9092"
	brokerTopicName = "messages"
	// DB config
	dbURI = "mongodb://user:pass@localhost:27017"
)

// Message struct
type Message struct {
	ID   uuid.UUID `json:"id" bson:"id,omitempty"`
	Text string    `json:"text" bson:"text,omitempty"`
}

func main() {
	ctx := context.Background()

	// Configure env
	envBrokerURL := os.Getenv(envBrokerURLName)
	if envBrokerURL != "" {
		brokerURL = envBrokerURL
	}
	envDBURI := os.Getenv(envDBURIName)
	if envDBURI != "" {
		dbURI = envDBURI
	}

	// Connect to kafka
	worker, err := connectConsumer([]string{brokerURL})
	if err != nil {
		log.Fatal(err)
	}

	// Calling ConsumePartition. It will open one connection per broker
	// and share it for all partitions that live on it.
	consumer, err := worker.ConsumePartition(brokerTopicName, 0, sarama.OffsetOldest)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to DB
	dbClient, err := connectToDB(ctx, dbURI)
	if err != nil {
		log.Fatal(err)
	}

	// Get DB collection
	collection := dbClient.Database("reader-db").Collection("messages")

	// Start http server for metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Println(err)
		}
	}()

	log.Println("Reader started")

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Get signal for finish
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case err = <-consumer.Errors():
				log.Println(err)
			case msg := <-consumer.Messages():
				err = handleConsumerMessage(ctx, collection, msg)
				if err != nil {
					log.Println(err)
					continue
				}
			case <-sigchan:
				fmt.Println("Interrupt is detected")
				doneCh <- struct{}{}
			}
		}
	}()

	<-doneCh

	err = worker.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func handleConsumerMessage(
	ctx context.Context,
	collection *mongo.Collection,
	msg *sarama.ConsumerMessage,
) error {
	log.Printf("Received message: Topic(%s) | Message(%s) \n", msg.Topic, string(msg.Value))

	result := "not_ok"
	defer func() {
		messageResult.WithLabelValues(result).Inc()
	}()

	msgToStore := &Message{}
	err := json.Unmarshal(msg.Value, msgToStore)
	if err != nil {
		return err
	}

	// Store message in DB
	insertResult, err := collection.InsertOne(ctx, msgToStore)
	if err != nil {
		return err
	}

	log.Printf("Store message id=%s in DB with inser_id=%s \n", msgToStore.ID, insertResult.InsertedID)

	// Use random for different metric values metric (80% ok and 20% not_ok)
	if rand.Float32() <= 0.8 {
		result = "ok"
	}

	return nil
}

func connectConsumer(brokersUrl []string) (sarama.Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	// Create new consumer
	conn, err := sarama.NewConsumer(brokersUrl, config)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func connectToDB(ctx context.Context, uri string) (*mongo.Client, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.Println("Connected to MongoDB!")

	return client, nil
}
