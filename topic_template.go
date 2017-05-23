package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"text/template"
)

var ErrTopicNameChars = errors.New("Topic name has abnormal characters.")
var ErrTopicNameLength = errors.New("Topic name is too long.")
var ErrTopicNameInvalid = errors.New("Topic name is invalid.")

const maxTopicLen = 249
const DefaultTopicTemplate = "telepath-influx-metrics"

var topicCharMatcher = regexp.MustCompile(`[a-zA-Z0-9\._\-]`)

type topicTemplate struct {
	tmpl *template.Template
}

type topicParams struct {
	Database string
}

func NewTopicTemplate(text string) (*topicTemplate, error) {
	if text == "" {
		text = DefaultTopicTemplate
	}

	tmpl := template.New("")
	tmpl.Funcs(template.FuncMap{
		"toLower": func(s string) (string, error) {
			return strings.ToLower(s), nil
		},
		"toUpper": func(s string) (string, error) {
			return strings.ToUpper(s), nil
		},
	})

	tmpl, err := tmpl.Parse(text)
	if err != nil {
		return nil, err
	}

	return &topicTemplate{tmpl}, nil
}

func (tf *topicTemplate) Execute(db string) (string, error) {
	var w bytes.Buffer
	params := topicParams{db}

	if err := tf.tmpl.Execute(&w, params); err != nil {
		return "", err
	}

	topic := string(w.Bytes())
	if err := validateTopicName(topic); err != nil {
		return "", err
	}

	return topic, nil
}

// https://github.com/apache/kafka/blob/trunk/core/src/main/scala/kafka/common/Topic.scala#L24
func validateTopicName(topic string) error {
	if len(topic) > maxTopicLen {
		return ErrTopicNameLength
	}

	if len(topic) == 0 || topic == "." || topic == ".." {
		return ErrTopicNameInvalid
	}

	for _, c := range []byte(topic) {
		if !topicCharMatcher.Match([]byte{c}) {
			return ErrTopicNameChars
		}
	}

	return nil
}
