package main

import (
	"bytes"
	"io"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/oxtoacart/bpool"
	"github.com/valyala/fasthttp"
)

func pingHandlerFunc(ctx *fasthttp.RequestCtx) {
	ctx.Response.SetStatusCode(http.StatusNoContent)
}

// Deliver a dummy response to the query endpoint, as some InfluxDB
// clients test endpoint availability with a query
func queryHandlerFunc(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("X-Influxdb-Version", "1.0")
	ctx.SetStatusCode(http.StatusOK)
	ctx.SetBody([]byte(`{"results":[]}`))
}

const MaxBodySize = 500 * 1024 * 1024
const MaxLineSize = 64 * 1024
const BufferPoolSize = 64

type writeHandler struct {
	bufPool     *bpool.BytePool
	maxBodySize int
	producer    sarama.AsyncProducer
	tf          *topicFormatter
}

type writeConfig struct {
	maxBodySize    int
	maxLineSize    int
	maxChunkSize   int
	bufferPoolSize int
	topicFormat    string
	topicCasing    string
}

func NewWriteHandler(producer sarama.AsyncProducer, config writeConfig) *writeHandler {
	maxBodySize := config.maxBodySize
	if maxBodySize < 1 {
		maxBodySize = MaxBodySize
	}
	maxLineSize := config.maxLineSize
	if maxLineSize < 1 {
		maxLineSize = MaxLineSize
	}
	maxChunkSize := config.maxChunkSize
	if maxChunkSize < 1 || maxChunkSize > maxLineSize {
		maxChunkSize = maxLineSize
	}
	bufferPoolSize := config.bufferPoolSize
	if bufferPoolSize < 1 {
		bufferPoolSize = BufferPoolSize
	}

	return &writeHandler{
		maxBodySize: maxBodySize,
		bufPool:     bpool.NewBytePool(bufferPoolSize, maxChunkSize),
		producer:    producer,
		tf:          NewTopicFormatter(config.topicFormat, config.topicCasing),
	}
}

func (wh *writeHandler) Handle(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}

	if ctx.Request.Header.ContentLength() > wh.maxBodySize {
		ctx.SetStatusCode(http.StatusRequestEntityTooLarge)
		return
	}

	var reader io.Reader
	contentEncoding := ctx.Request.Header.Peek("Content-Encoding")
	if string(contentEncoding) != "gzip" {
		reader = bytes.NewReader(ctx.Request.Body())
	} else {
		body, err := ctx.Request.BodyGunzip()
		if err != nil {
			ctx.SetStatusCode(http.StatusBadRequest)
			return
		}

		reader = bytes.NewReader(body)
	}

	topic, err := wh.tf.Format("foo")
	if err != nil {
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}

	buffer := wh.bufPool.Get()
	defer wh.bufPool.Put(buffer)

	parser := NewLineParser(buffer, "s")
	for {
		line, err := parser.Next(reader)
		if err == io.EOF {
			break
		}

		//fmt.Printf("Writing to topic %s\n", topic)

		wh.producer.Input() <- &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(line),
		}
	}

	ctx.SetStatusCode(http.StatusNoContent)
}
