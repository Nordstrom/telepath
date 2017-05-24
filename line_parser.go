package main

import (
	"bytes"
	"io"
	"strings"
	"strconv"
	"time"
)

var emptyBuffer = []byte{}

type lineParser struct {
	position  int
	offset    int
	length    int
	precision string
	buffer    []byte
}

func NewLineParser(buffer []byte, precision string) *lineParser {
	return &lineParser{
		buffer:    buffer,
		precision: precision,
	}
}

func parseComplete(err error) bool {
	return err != nil && err != io.ErrUnexpectedEOF
}

func (lp *lineParser) Next(reader io.Reader) ([]byte, error) {
	line := emptyBuffer
	for {
		var err error
		if lp.position == 0 {
			lp.length, err = io.ReadFull(reader, lp.buffer[lp.offset:])
			if parseComplete(err) {
				return emptyBuffer, err
			}
		}

		tail := bytes.IndexByte(lp.buffer[lp.position:], '\n')
		if lp.length <= lp.position {
			// We've over-run the usable data in our buffer!
			return emptyBuffer, io.EOF
		} else if tail != -1 {
			// We've found a metric!
			next := lp.position + tail + 1
			line = lp.buffer[lp.position: next-1]
			lp.position = next

			break
		} else if err == io.ErrUnexpectedEOF {
			// We're at the end of the line,
			// and there's no remaining input.
			tail = lp.offset + lp.length
			line = lp.buffer[:tail]
			lp.position = tail

			break
		} else {
			// We'll rotate the remainder of this chunk,
			// making it available for the next iteration.
			remainder := len(lp.buffer) - lp.position
			copy(lp.buffer, lp.buffer[lp.position:])

			lp.position = 0
			lp.offset = remainder
		}
	}

	return convertToNanoseconds(line, lp.precision), nil
}

func convertToNanoseconds(input []byte, precision string) ([]byte) {
	values := strings.Split(string(input[:]), " ")
	var multiplyer time.Duration
	switch precision {
	case "ns":
		multiplyer = time.Nanosecond
	case "us":
		multiplyer = time.Microsecond
	case "ms":
		multiplyer = time.Millisecond
	case "s":
		multiplyer = time.Second
	default:
		multiplyer = time.Nanosecond
	}

	t, err := strconv.ParseInt(values[len(values)-1], 10, 64)
	if err != nil {
		values = append(values, strconv.FormatInt(time.Now().UnixNano(), 10))
	} else {
		values = values[:len(values)-1]
		values = append(values, strconv.FormatInt(t * int64(multiplyer), 10))
	}
	return []byte(strings.Join(values[:], " "))
}