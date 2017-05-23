package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_topic_template(t *testing.T) {
	cases := []struct {
		label    string
		db       string
		template string
		expect   string
		err      error
	}{
		{
			label:  "Empty template",
			db:     "foo",
			expect: DefaultTopicTemplate,
		}, {
			label:    "Hard-coded name",
			db:       "foo",
			template: "foo-topic",
			expect:   "foo-topic",
		}, {
			label:    "Templated name",
			db:       "foo",
			template: "{{.Database}}-topic",
			expect:   "foo-topic",
		}, {
			label:    "Template toLower",
			db:       "Foo",
			template: "{{.Database|toLower}}-topic",
			expect:   "foo-topic",
		}, {
			label:    "Template toUpper",
			db:       "Foo",
			template: "{{.Database|toUpper}}-topic",
			expect:   "FOO-topic",
		}, {
			label:    "Bad chars in db",
			db:       "#&*",
			template: "{{.Database}}-topic",
			err:      ErrTopicNameChars,
		}, {
			label:    "Bad chars in template",
			db:       "foo",
			template: "{{.Database}}-#&*",
			err:      ErrTopicNameChars,
		}, {
			label:    "Topic name too long",
			db:       "foo",
			template: strings.Repeat("x", maxTopicLen+1),
			err:      ErrTopicNameLength,
		}, {
			label:    "Bad name",
			db:       "foo",
			template: "..",
			err:      ErrTopicNameInvalid,
		},
	}

	for _, c := range cases {
		tf, err := NewTopicTemplate(c.template)
		assert.NoError(t, err)

		topic, err := tf.Execute(c.db)
		if c.err == nil {
			assert.NoError(t, err)
			assert.Equal(t, c.expect, topic)
		} else {
			assert.Empty(t, topic)
			assert.Error(t, err)
		}
	}
}
