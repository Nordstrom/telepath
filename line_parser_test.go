package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_line_parser_parsing(t *testing.T) {
	cases := []struct {
		label  string
		input  string
		expect []string
	}{
		//		{
		//			label:  "No metrics",
		//			input:  "",
		//			expect: []string{},
		//		},
		//		{
		//			label: "Single metric",
		//			input: "foo value=1 1\n",
		//			expect: []string{
		//				"foo value=1 1",
		//			},
		//		},
		{
			label: "Many metrics",
			input: "foo,host=Coruscant value=1 1\nfoo,host=Tatooine value=1 1\nfoo,host=Alderaan value=1 1\nfoo,host=Hoth value=1 1\nfoo,host=Naboo value=1 1\nfoo,host=Bespin value=1 1\nfoo,host=Dagobah value=1 1\n",
			expect: []string{
				"foo,host=Coruscant value=1 1",
				"foo,host=Tatooine value=1 1",
				"foo,host=Alderaan value=1 1",
				"foo,host=Hoth value=1 1",
				"foo,host=Naboo value=1 1",
				"foo,host=Bespin value=1 1",
				"foo,host=Dagobah value=1 1",
			},
		},
		//		{
		//			label: "Metric without trailing newline",
		//			input: "foo value=1 1",
		//			expect: []string{
		//				"foo value=1 1",
		//			},
		//		},
	}

	for _, c := range cases {
		buffer := make([]byte, 32)
		lp := NewLineParser(buffer, "")
		reader := bytes.NewBuffer([]byte(c.input))

		actual := make([]string, 0)
		for {
			line, err := lp.Next(reader)
			if err != nil {
				if err != io.EOF {
					t.Fatalf("%s: Expected EOF", c.label)
				}
				break
			}
			actual = append(actual, string(line))
		}
		assert.Equal(t, c.expect, actual, c.label)
	}
}

func Test_line_parser_upscaling(t *testing.T) {
	cases := []struct {
		label     string
		precision string
		input     string
		expect    string
	}{
		{
			label:     "Blank precision",
			precision: "",
			input:     "foo value=1 1",
			expect:    "foo value=1 1",
		},
		{
			label:     "Nanoscond precision",
			precision: "ns",
			input:     "foo value=1 1",
			expect:    "foo value=1 1",
		},
		{
			label:     "Microsecond precision",
			precision: "us",
			input:     "foo value=1 1",
			expect:    "foo value=1 1000",
		},
		{
			label:     "Millisecond precision",
			precision: "ms",
			input:     "foo value=1 1",
			expect:    "foo value=1 1000000",
		},
		{
			label:     "Second precision",
			precision: "s",
			input:     "foo value=1 1",
			expect:    "foo value=1 1000000000",
		},
		{
			label:     "Second precision",
			precision: "s",
			input:     "foo value=1 1",
			expect:    "foo value=1 1000000000",
		},
	}

	for _, c := range cases {
		buffer := make([]byte, 32)
		lp := NewLineParser(buffer, c.precision)
		reader := bytes.NewBuffer([]byte(c.input))

		actual, err := lp.Next(reader)
		assert.NoError(t, err)
		assert.Equal(t, c.expect, string(actual), c.label)
	}
}

func Test_line_parser_appends_missing_timestamp(t *testing.T) {
	expected := []string{"foo,x=y", "value=1"}

	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1\n"))

	actual, err := lp.Next(reader)
	actualValues := strings.Split(string(actual[:]), " ")
	require.Equal(t, len(actualValues), 3, "Timestamp was not added to message")
	require.Equal(t, expected[0], actualValues[0], "First key value was altered")
	require.Equal(t, expected[1], actualValues[1], "Second key value was altered")
	actualTime, err := strconv.ParseInt(actualValues[2], 10, 64)
	require.True(t, actualTime > time.Now().UnixNano()-int64(time.Millisecond*2), "Time was not generated within the last 2 milliseconds")
	require.NoError(t, err, "Error while parsing time as integer")
	require.Nil(t, err)

	actual, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, actual)
	require.Equal(t, io.EOF, err)
}
