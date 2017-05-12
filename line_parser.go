package main

import (
	"bytes"
	"io"
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

func (lp *lineParser) Next(reader io.Reader) ([]byte, error) {
	line := emptyBuffer
	for {
		if lp.position == 0 {
			var err error
			if lp.length, err = reader.Read(lp.buffer[lp.offset:]); err != nil {
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
			line = lp.buffer[lp.position : next-1]
			lp.position = next

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

	return line, nil
}
