package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"time"
)

var emptyBuffer = []byte{}

type lineParser struct {
	position  int
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
	var offset int
	var length int
	line := emptyBuffer
	for {
		var err error
		if lp.position == 0 {
			length, err = io.ReadFull(reader, lp.buffer[offset:])
			lp.length = offset + length
			if parseComplete(err) {
				return emptyBuffer, err
			}
		}

		tail := bytes.IndexByte(lp.buffer[lp.position:], '\n')
		if lp.length < lp.position {
			// We've over-run the usable data in our buffer!
			return emptyBuffer, io.EOF
		} else if tail != -1 {
			// We've found a metric!
			next := lp.position + tail + 1
			line = lp.buffer[lp.position:next]
			lp.position = next

			break
		} else if err == io.ErrUnexpectedEOF {
			// We're at the end of the line,
			// and there's no remaining input.
			tail = offset + lp.length
			line = lp.buffer[:tail]
			lp.position = tail

			break
		} else {
			// We'll rotate the remainder of this chunk,
			// making it available for the next iteration.
			remainder := len(lp.buffer) - lp.position
			copy(lp.buffer, lp.buffer[lp.position:])

			lp.position = 0
			offset = remainder
		}
	}

	return convertToNanoseconds(trimNewline(line), lp.precision), nil
}

func trimNewline(line []byte) []byte {
	length := len(line)
	if c := line[length-1]; c != '\n' {
		return line
	}
	return line[:length-1]
}

func convertToNanoseconds(input []byte, precision string) []byte {
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
		values = append(values, strconv.FormatInt(t*int64(multiplyer), 10))
	}
	return []byte(strings.Join(values[:], " "))
}
