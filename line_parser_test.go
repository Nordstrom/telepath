package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
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
	reader := bytes.NewBuffer([]byte("foo,x=y value=1\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}