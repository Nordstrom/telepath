package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Nordstrom/telepath/middleware"
	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
	"github.com/buaazp/fasthttprouter"
	"github.com/valyala/fasthttp"
)

func main() {
	config := &TelepathConfig{}
	config.Parse()

	if config.Brokers == "" {
		log.Fatal("Please specify at least one Kafka broker")
	}

	kafkaClient, err := newKafkaClient(strings.Split(config.Brokers, ","), time.Minute, config.Version)
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

	router := fasthttprouter.New()
	router.GET("/ping", pingHandlerFunc)
	router.GET("/query", middleware.Auth(queryHandlerFunc, &config.Auth))
	router.POST("/query", middleware.Auth(queryHandlerFunc, &config.Auth))
	router.POST("/write", middleware.Auth(write.Handle, &config.Auth))
	router.GET("/metrics", metrics.Handle)

	server := &fasthttp.Server{
		Name:               "Telepath InfluxDB endpoint",
		MaxRequestBodySize: MaxBodySize,
		Handler:            router.Handler,
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

func newKafkaClient(brokers []string, timeout time.Duration, version sarama.KafkaVersion) (client sarama.Client, err error) {
	config := sarama.NewConfig()

	config.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	config.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	config.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms
	config.Producer.Return.Successes = true
	config.Version = version

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
	serverCertificate, err := tls.LoadX509KeyPair(config.CertificatePath, config.KeyPath)
	if err != nil {
		log.Fatalf("Could not load server certificate %s: %v", config.CertificatePath, err)
	}

	var clientAuth tls.ClientAuthType
	var clientCertPool *x509.CertPool
	switch config.ClientVerify {
	case "optional":
		clientAuth = tls.RequestClientCert
		break
	case "required":
		clientAuth = tls.RequireAndVerifyClientCert
		break
	}

	if clientAuth != tls.NoClientCert {
		clientCertPool = x509.NewCertPool()
		for _, certificatePath := range config.ClientCertificatePaths {
			certBytes, err := ioutil.ReadFile(certificatePath)
			if err != nil {
				log.Fatalf("Could not load client certificate %s: %v", certificatePath, err)
			}

			block, certBytes := pem.Decode(certBytes)
			clientCertificate, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				log.Fatalf("Could not parse client certificate %s: %v", certificatePath, err)
			}

			log.Debugf("Adding client certificate %s", certificatePath)

			clientCertPool.AddCert(clientCertificate)
		}
	}

	listener, err := tls.Listen("tcp4", config.Addr, &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		ClientCAs:    clientCertPool,
		ClientAuth:   clientAuth,
	})

	if err != nil {
		log.Fatalf("Could not setup tls config: %v", err)
	}

	log.Infof("Starting Telepath server: %v", listener.Addr())
	go func(litener net.Listener) {
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
