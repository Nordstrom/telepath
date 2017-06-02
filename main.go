package main

import (
	"fmt"
	"strings"
	"time"

	"flag"

	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

type TelepathConfig struct {
	TopicTemplate string
	Brokers       string
	LogLevel      string
	LogFormat     string
}

func main() {
	config := TelepathConfig{}
	port := flag.Int("http.port", 8089, "An HTTP port to bind to")

	flag.StringVar(&config.Brokers,
		"brokers", "", "A comma-separated list of Kafka host:port addrs to connect to")
	flag.StringVar(&config.TopicTemplate,
		"topic.name", DefaultTopicTemplate, "The Kafka topic name/template to write metrics to")
	flag.StringVar(&config.LogLevel,
		"log.level", log.InfoLevel.String(), "Logging level: debug, info, warning, error")
	flag.StringVar(&config.LogFormat,
		"log.format", LogFormatText, "Logging format: text, json")
	flag.Parse()

	SetLogFormat(config.LogFormat)
	SetLogLevel(config.LogLevel)

	if config.Brokers == "" {
		log.Fatal("Please specify at least one Kafka broker")
	}

	kafkaClient, err := newKafkaClient(strings.Split(config.Brokers, ","), time.Minute)
	if err != nil {
		log.Fatalf("Could not connect to Kafka brokers: %v", err)
	}

	kafkaProducer, err := newKafkaProducer(kafkaClient)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}

	listener, err := reuseport.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Could not open port: %v", err)
	}

	write, err := NewWriteHandler(kafkaProducer, writeConfig{
		topicTemplate: config.TopicTemplate,
	})

	if err != nil {
		log.Fatalf("Could not create handler: %v", err)
	}

	server := &fasthttp.Server{
		Name:               "Telepath InfluxDB endpoint",
		MaxRequestBodySize: MaxBodySize,
		Handler: func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/ping":
				pingHandlerFunc(ctx)
			case "/query":
				queryHandlerFunc(ctx)
			case "/write":
				write.Handle(ctx)
			case "/metrics":
				metrics.Handle(ctx)
			default:
				ctx.NotFound()
			}
		},
	}

	log.Infof("Starting Telepath server: %v", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
}

func newKafkaProducer(client sarama.Client) (sarama.AsyncProducer, error) {
	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}

	// We will log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	go func() {
		for err := range producer.Errors() {
			log.Errorf("Error processing line: %v", err)
		}
	}()

	return producer, nil
}

func newKafkaClient(brokers []string, timeout time.Duration) (client sarama.Client, err error) {
	config := sarama.NewConfig()

	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms

	retryTimeout := time.Duration(10 * time.Second)
	for {
		client, err = sarama.NewClient(brokers, config)
		if err == nil {
			log.Infof("Connected to Kafka: %s", strings.Join(brokers, ","))
			break
		}

		log.Errorf("Couldn't connect to Kafka! Trying again in %v. %v", retryTimeout, err)
		time.Sleep(retryTimeout)
	}

	return
}
