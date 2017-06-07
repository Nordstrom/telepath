package main

import (
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func main() {
	config := &TelepathConfig{}
	config.Parse()

	if config.Brokers == "" {
		log.Fatal("Please specify at least one Kafka broker")
	}

	kafkaClient, err := newKafkaClient(strings.Split(config.Brokers, ","), time.Minute)
	if err != nil {
		log.Fatalf("Could not connect to Kafka brokers: %v", err)
	}

	kafkaProducer, err := sarama.NewAsyncProducerFromClient(kafkaClient)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}

	write, err := NewWriteHandler(kafkaProducer, writeConfig{
		topicTemplate: config.TopicTemplate,
	})

	if err != nil {
		log.Fatalf("Could not create a write handler: %v", err)
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

	doneCh := make(chan bool)
	go followProducer(kafkaProducer, doneCh)

	wg := &sync.WaitGroup{}
	if config.HTTP.Enabled {
		go serveHTTP(server, &config.HTTP, wg, doneCh)
	}
	if config.HTTPS.Enabled {
		go serveHTTPS(server, &config.HTTPS, wg, doneCh)
	}

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh,
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-signalCh
	log.Infof("Shutting down...")

	close(doneCh)
	wg.Wait()
}

func newKafkaClient(brokers []string, timeout time.Duration) (client sarama.Client, err error) {
	config := sarama.NewConfig()

	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms
	config.Producer.Return.Successes = true

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

func serveHTTP(server *fasthttp.Server, config *HTTPConfig, wg *sync.WaitGroup, doneCh chan bool) {
	listener, err := net.Listen("tcp4", config.Addr)
	if err != nil {
		log.Fatalf("Could not start %s listener: %v", config.Addr, err)
	}

	log.Infof("Starting Telepath server: %v", listener.Addr())
	go func(listener net.Listener) {
		wg.Add(1)
		defer wg.Done()

		if err := server.Serve(listener); err != nil {
			log.Fatalf("Unexpected error: %v", err)
		}

		log.Infof("Stopped Telepath server: %v", listener.Addr())
	}(listener)

	<-doneCh
	listener.Close()
}

func serveHTTPS(server *fasthttp.Server, config *HTTPSConfig, wg *sync.WaitGroup, doneCh chan bool) {
	listener, err := net.Listen("tcp4", config.Addr)
	if err != nil {
		log.Fatalf("Could not start %s listener: %v", config.Addr, err)
	}

	log.Infof("Starting Telepath server: %v", listener.Addr())
	go func(litener net.Listener) {
		wg.Add(1)
		defer wg.Done()

		if err := server.ServeTLS(listener, config.CertificatePath, config.KeyPath); err != nil {
			log.Fatalf("Unexpected error: %v", err)
		}

		log.Infof("Stopped Telepath server: %v", listener.Addr())
	}(listener)

	<-doneCh
	listener.Close()
}

func followProducer(producer sarama.AsyncProducer, doneCh chan bool) {
	for {
		select {
		case <-doneCh:
			return

		case err := <-producer.Errors():
			msg := err.Msg
			metrics.KafkaProducerErrorCount(msg.Topic).Inc()

			line, _ := msg.Value.Encode()
			log.WithFields(log.Fields{
				"line":  string(line),
				"topic": msg.Topic,
			}).Debugf("Unable to produce a line to the '%s' topic: %v", msg.Topic, err.Err)

		case msg := <-producer.Successes():
			metrics.KafkaProducerSuccessCount(msg.Topic).Inc()

			line, _ := msg.Value.Encode()
			log.WithFields(log.Fields{
				"line":  string(line),
				"topic": msg.Topic,
			}).Debugf("Produced a line to the '%s' topic.", msg.Topic)
		}
	}
}
