package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_line_parser_no_data(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "s")
	reader := bytes.NewBuffer([]byte{})

	line, err := lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_single_metric(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "s")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1494462271\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1494462271", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_without_trailing_newline(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "s")
	reader := bytes.NewBuffer([]byte("foo,x=y value=1 1494462271"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1494462271", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}

func Test_line_parser_multiple_metrics(t *testing.T) {
	buffer := make([]byte, 32)
	lp := NewLineParser(buffer, "s")
	reader := bytes.NewBuffer([]byte(
		"foo,x=y value=1 1494462271\nbar,x=y value=1 1494462271\n"))

	line, err := lp.Next(reader)
	require.Equal(t, "foo,x=y value=1 1494462271", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, "bar,x=y value=1 1494462271", string(line))
	require.Nil(t, err)

	line, err = lp.Next(reader)
	require.Equal(t, emptyBuffer, line)
	require.Equal(t, io.EOF, err)
}
