package main

import (
	"flag"
	"fmt"

	"github.com/Nordstrom/telepath/middleware"
	log "github.com/Sirupsen/logrus"
)

type TelepathConfig struct {
	Brokers       string
	TopicTemplate string
	LogLevel      string
	LogFormat     string
	HTTP          HTTPConfig
	HTTPS         HTTPSConfig
	Auth          middleware.AuthConfig
}

type HTTPConfig struct {
	Enabled bool
	Addr    string
}

type HTTPSConfig struct {
	Enabled                bool
	Addr                   string
	CertificatePath        string
	KeyPath                string
	ClientVerify           string
	ClientCertificatePaths []string
}

type stringSlice []string

func (c *TelepathConfig) Parse() {
	var clientCertificatePaths stringSlice

	flag.StringVar(&c.Brokers, "brokers", "", "A comma-separated list of Kafka host:port addrs to connect to")
	flag.StringVar(&c.TopicTemplate, "topic.name", DefaultTopicTemplate, "The Kafka topic name/template to write metrics to")

	flag.StringVar(&c.HTTP.Addr, "http.addr", ":8089", "An HTTP addr to bind to")
	flag.BoolVar(&c.HTTP.Enabled, "http.enabled", true, "Listen to HTTP addr, if true")

	flag.StringVar(&c.HTTPS.Addr, "https.addr", ":8090", "An HTTPS addr to bind to")
	flag.BoolVar(&c.HTTPS.Enabled, "https.enabled", false, "Listen to HTTP addr, if true")
	flag.StringVar(&c.HTTPS.CertificatePath, "https.certificate", "", "Path to a TLS certificate file")
	flag.StringVar(&c.HTTPS.KeyPath, "https.key", "", "Path to a TLS key file")
	flag.StringVar(&c.HTTPS.ClientVerify, "https.client.verify", "none", "Client certificate verification: none, optional, or required")
	flag.Var(&clientCertificatePaths, "https.client.certificate", "Path to a client certificate file")

	flag.BoolVar(&c.Auth.Enabled, "auth.enabled", false, "Authenticate user, if true")
	flag.StringVar(&c.Auth.Username, "auth.username", "", "Name of authenticated user")
	flag.StringVar(&c.Auth.Password, "auth.password", "", "Password of authenticated user")

	flag.StringVar(&c.LogLevel, "log.level", log.InfoLevel.String(), "Logging level: debug, info, warning, error")
	flag.StringVar(&c.LogFormat, "log.format", LogFormatText, "Logging format: text, json")
	flag.Parse()

	c.HTTPS.ClientCertificatePaths = make([]string, len(clientCertificatePaths))
	copy(c.HTTPS.ClientCertificatePaths, clientCertificatePaths)

	SetLogFormat(c.LogFormat)
	SetLogLevel(c.LogLevel)
}

func (ss *stringSlice) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *stringSlice) Set(value string) error {
	if value != "" {
		*ss = append(*ss, value)
	}
	return nil
}
