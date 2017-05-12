package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_topic_formatter(t *testing.T) {
	cases := []struct {
		label  string
		db     string
		format string
		casing string
		expect string
		err    error
	}{
		{
			label:  "No formatting",
			db:     "foo",
			format: "my-topic",
			casing: CasingNone,
			expect: "my-topic",
		}, {
			label:  "Prefix",
			db:     "foo",
			format: "*-topic",
			casing: CasingNone,
			expect: "foo-topic",
		}, {
			label:  "Suffix",
			db:     "foo",
			format: "topic-*",
			casing: CasingNone,
			expect: "topic-foo",
		}, {
			label:  "Lower-casing",
			db:     "Foo",
			format: "*-topic",
			casing: CasingLowercase,
			expect: "foo-topic",
		}, {
			label:  "Upper-casing",
			db:     "foo",
			format: "*-topic",
			casing: CasingUppercase,
			expect: "FOO-TOPIC",
		}, {
			label:  "Bad chars in db",
			db:     "#&*",
			format: "*-topic",
			casing: CasingNone,
			err:    ErrTopicNameChars,
		}, {
			label:  "Bad chars in format",
			db:     "foo",
			format: "*-#&*",
			casing: CasingNone,
			err:    ErrTopicNameChars,
		}, {
			label:  "Topic name too long",
			db:     "foo",
			format: strings.Repeat("x", maxTopicLen+1),
			casing: CasingNone,
			err:    ErrTopicNameLength,
		}, {
			label:  "Empty format",
			db:     "foo",
			format: "",
			casing: CasingNone,
			err:    ErrTopicNameInvalid,
		}, {
			label:  "Bad format",
			db:     "foo",
			format: "..",
			casing: CasingNone,
			err:    ErrTopicNameInvalid,
		},
	}

	for _, c := range cases {
		tf := topicFormatter{
			format: c.format,
			casing: c.casing,
		}
		topic, err := tf.Format(c.db)
		if c.err == nil {
			assert.NoError(t, err)
			assert.Equal(t, c.expect, topic)
		} else {
			assert.Empty(t, topic)
			assert.Error(t, err)
		}
	}
}
