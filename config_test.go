package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/Shopify/sarama"
)

func Test_version_parsing(t *testing.T) {
	cases := []struct {
		label   string
		input   string
		expect  sarama.KafkaVersion
	}{
		{
			label:   "empty input",
			input:   "",
			expect:  sarama.V0_10_0_0,
		},
		{
			label:   "Case should not matter",
			input:   "v0_11_0_0",
			expect:  sarama.V0_11_0_0,
		},
		{
			label:   "V0_11_0_0",
			input:   "V0_11_0_0",
			expect:  sarama.V0_11_0_0,
		},
		{
			label:   "V0_10_2_0",
			input:   "V0_10_2_0",
			expect:  sarama.V0_10_2_0,
		},
		{
			label:   "V0_10_1_0",
			input:   "V0_10_1_0",
			expect:  sarama.V0_10_1_0,
		},
		{
			label:   "V0_10_0_0",
			input:   "V0_10_0_0",
			expect:  sarama.V0_10_0_0,
		},
		{
			label:   "V0_9_0_1",
			input:   "V0_9_0_1",
			expect:  sarama.V0_9_0_1,
		},
		{
			label:   "V0_9_0_0",
			input:   "V0_9_0_0",
			expect:  sarama.V0_9_0_0,
		},
		{
			label:   "V0_8_2_2",
			input:   "V0_8_2_2",
			expect:  sarama.V0_8_2_2,
		},
		{
			label:   "V0_8_2_1",
			input:   "V0_8_2_1",
			expect:  sarama.V0_8_2_1,
		},
		{
			label:   "V0_8_2_0",
			input:   "V0_8_2_0",
			expect:  sarama.V0_8_2_0,
		},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			actual := StringToKafkaVersion(&c.input)
			assert.Equal(t, c.expect, actual)
		})
	}
}

