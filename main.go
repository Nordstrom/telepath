package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"flag"

	"github.com/Shopify/sarama"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

type TelepathConfig struct {
	TopicFormat string
	TopicCasing string
	Brokers     string
}

func main() {
	config := TelepathConfig{}
	port := flag.Int("http.port", 8089, "An HTTP port to bind to")

	flag.StringVar(&config.Brokers,
		"brokers", "", "A comma-separated list of Kafka host:port addrs to connect to")
	flag.StringVar(&config.TopicFormat,
		"topic.format", "telepath-metrics", "The Kafka topic name/format to write metrics to")
	flag.StringVar(&config.TopicCasing,
		"topic.casing", CasingNone, "The casing to apply to the Kafka topic name/format")
	flag.Parse()

	if config.Brokers == "" {
		fmt.Printf("Please specify at least one Kafka broker")
		os.Exit(1)
	}

	listener, err := reuseport.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		fmt.Printf("Could not open port: %v", err)
		return
	}

	producer := newProducer(strings.Split(config.Brokers, ","))
	write := NewWriteHandler(producer, writeConfig{
		topicFormat: config.TopicFormat,
		topicCasing: config.TopicCasing,
	})

	server := &fasthttp.Server{
		Name: "Telepath InfluxDB endpoint",
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

	fmt.Printf("Starting Telepath server: %v\n", listener.Addr())
	if err := server.Serve(listener); err != nil {
		fmt.Printf("Unexpected error: %v", err)
	}
}

func newProducer(brokers []string) sarama.AsyncProducer {
	config := sarama.NewConfig()

	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		log.Fatalln("Failed to start Kafka producer:", err)
	}

	// We will just log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	go func() {
		for err := range producer.Errors() {
			log.Printf("Error processing line: %v", err)
		}
	}()

	return producer
}
