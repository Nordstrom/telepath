package main

import (
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"flag"

	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport"
)

type TelepathConfig struct {
	Brokers       string
	TopicTemplate string
	LogLevel      string
	LogFormat     string
	HTTPAddr      string
	HTTPCert      string
	HTTPKey       string
}

func main() {
	config := TelepathConfig{}
	flag.StringVar(&config.Brokers, "brokers", "", "A comma-separated list of Kafka host:port addrs to connect to")
	flag.StringVar(&config.TopicTemplate, "topic.name", DefaultTopicTemplate, "The Kafka topic name/template to write metrics to")
	flag.StringVar(&config.HTTPAddr, "http.addr", ":8089", "An HTTP addr to bind to")
	flag.StringVar(&config.HTTPCert, "http.cert", "", "Path to a TLS certificate file")
	flag.StringVar(&config.HTTPKey, "http.key", "", "Path to a TLS key file")
	flag.StringVar(&config.LogLevel, "log.level", log.InfoLevel.String(), "Logging level: debug, info, warning, error")
	flag.StringVar(&config.LogFormat, "log.format", LogFormatText, "Logging format: text, json")
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

	kafkaProducer, err := sarama.NewAsyncProducerFromClient(kafkaClient)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}

	listener, err := reuseport.Listen("tcp4", config.HTTPAddr)
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

	doneCh := make(chan bool)

	go followProducer(kafkaProducer, doneCh)
	go handleShutdown(listener, doneCh)
	if err := serveRequests(server, listener, config.HTTPCert, config.HTTPKey); err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
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

func serveRequests(server *fasthttp.Server, listener net.Listener, certFile, keyFile string) error {
	log.Infof("Starting Telepath server: %v", listener.Addr())
	if certFile != "" {
		return server.ServeTLS(listener, certFile, keyFile)
	} else {
		return server.Serve(listener)
	}
}

func handleShutdown(listener net.Listener, doneCh chan bool) {
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-signalCh
	log.Infof("Shutting down...")

	doneCh <- true
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
