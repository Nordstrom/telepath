package main

import (
	"errors"
	"regexp"
	"strings"
)

const CasingLowercase = "lower"
const CasingUppercase = "upper"
const CasingNone = "none"

var ErrTopicNameChars = errors.New("Topic name has abnormal characters.")
var ErrTopicNameLength = errors.New("Topic name is too long.")
var ErrTopicNameInvalid = errors.New("Topic name is invalid.")

const maxTopicLen = 249

var topicCharMatcher = regexp.MustCompile(`[a-zA-Z0-9\._\-]`)

type topicFormatter struct {
	format string
	casing string
}

func NewTopicFormatter(format, casing string) *topicFormatter {
	if casing == "" {
		casing = CasingNone
	}

	return &topicFormatter{
		format: format,
		casing: casing,
	}
}

func (tf *topicFormatter) Format(db string) (string, error) {
	topic := strings.Replace(tf.format, "*", db, 1)

	if tf.casing == CasingLowercase {
		topic = strings.ToLower(topic)
	} else if tf.casing == CasingUppercase {
		topic = strings.ToUpper(topic)
	}

	return tf.validate(topic)
}

// https://github.com/apache/kafka/blob/trunk/core/src/main/scala/kafka/common/Topic.scala#L24
func (tf *topicFormatter) validate(topic string) (string, error) {
	if len(topic) > maxTopicLen {
		return "", ErrTopicNameLength
	}

	if len(topic) == 0 || topic == "." || topic == ".." {
		return "", ErrTopicNameInvalid
	}

	for _, c := range []byte(topic) {
		if !topicCharMatcher.Match([]byte{c}) {
			return "", ErrTopicNameChars
		}
	}

	return topic, nil
}
