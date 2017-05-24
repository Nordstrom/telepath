package main

import (
	"bytes"
	"io"
	"testing"
	"github.com/stretchr/testify/require"
	"strings"
	"strconv"
	"time"
)

func Test_line_parser_no_data(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ns")
	reader := bytes.NewBuffer([]byte{})

	line, err := lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_single_metric(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ns")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_without_trailing_newline(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ns")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_multiple_metrics(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ns")
	reader := bytes.NewBuffer([]byte(
		"foo,x=y value=1 1\nbar,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, "bar,x=y value=1 1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_not_upscale_when_nanoseconds_precision(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ns")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_upscale_microseconds_into_nanoseconds(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "us")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1000", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_upscale_milliseconds_into_nanoseconds(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "ms")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1000000", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_upscale_seconds_into_nanoseconds(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "s")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1000000000", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_no_change_timestamp_when_blank_precision(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1000\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1000", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_should_append_time_stamp_if_not_provided(t *testing.T) {
	expected := []string{ "foo,x=y", "value=1" }

	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1\n"))

	actual, err := lp.Next(reader)
	actualValues := strings.Split(string(actual[:]), " ")
	require.Equal(t, len(actualValues), 3, "Timestamp was not added to message")
	require.Equal(t, expected[0], actualValues[0], "First key value was altered")
	require.Equal(t, expected[1], actualValues[1], "Second key value was altered")
	actualTime, err := strconv.ParseInt(actualValues[2], 10, 64)
	require.True(t, actualTime > time.Now().UnixNano() - (Millisecond * 2), "Time was not generated within the last 2 milliseconds")
	require.NoError(t, err, "Error while parsing time as integer")
	require.Nil(t, err)

	actual, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, actual)
	require.Equal(t, io.EOF, err)
}