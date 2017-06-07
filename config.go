package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
)

type TelepathConfig struct {
	Brokers       string
	TopicTemplate string
	LogLevel      string
	LogFormat     string
	HTTP          HTTPConfig
	HTTPS         HTTPSConfig
}

type HTTPConfig struct {
	Enabled bool
	Addr    string
}

type HTTPSConfig struct {
	Enabled         bool
	Addr            string
	CertificatePath string
	KeyPath         string
}

func (c *TelepathConfig) Parse() {
	flag.StringVar(&c.Brokers, "brokers", "", "A comma-separated list of Kafka host:port addrs to connect to")
	flag.StringVar(&c.TopicTemplate, "topic.name", DefaultTopicTemplate, "The Kafka topic name/template to write metrics to")

	flag.StringVar(&c.HTTP.Addr, "http.addr", ":8089", "An HTTP addr to bind to")
	flag.BoolVar(&c.HTTP.Enabled, "http.enabled", true, "Listen to HTTP addr, if true")

	flag.StringVar(&c.HTTPS.Addr, "https.addr", ":8090", "An HTTPS addr to bind to")
	flag.BoolVar(&c.HTTPS.Enabled, "https.enabled", false, "Listen to HTTP addr, if true")
	flag.StringVar(&c.HTTPS.CertificatePath, "https.certificate", "", "Path to a TLS certificate file")
	flag.StringVar(&c.HTTPS.KeyPath, "https.key", "", "Path to a TLS key file")

	flag.StringVar(&c.LogLevel, "log.level", log.InfoLevel.String(), "Logging level: debug, info, warning, error")
	flag.StringVar(&c.LogFormat, "log.format", LogFormatText, "Logging format: text, json")
	flag.Parse()

	SetLogFormat(c.LogFormat)
	SetLogLevel(c.LogLevel)
}
