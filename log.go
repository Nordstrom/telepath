package main

import log "github.com/sirupsen/logrus"

const LogFormatText = "text"
const LogFormatJSON = "json"

func SetLogFormat(f string) {
	if f == LogFormatJSON {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}
}

func SetLogLevel(l string) {
	level, err := log.ParseLevel(l)
	if err != nil {
		log.Fatalf("Oops! %v", err)
	}

	log.SetLevel(level)
}
