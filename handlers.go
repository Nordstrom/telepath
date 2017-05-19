package main

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	"github.com/oxtoacart/bpool"
	"github.com/valyala/fasthttp"
)

func pingHandlerFunc(ctx *fasthttp.RequestCtx) {
	ctx.Response.SetStatusCode(http.StatusNoContent)
	metrics.PingRequestCount(ctx.Method(), http.StatusNoContent).Inc()
}

// Deliver a dummy response to the query endpoint, as some InfluxDB
// clients test endpoint availability with a query
func queryHandlerFunc(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("X-Influxdb-Version", "1.0")
	ctx.SetStatusCode(http.StatusOK)
	ctx.SetBody([]byte(`{"results":[]}`))

	metrics.QueryRequestCount(ctx.Method(), http.StatusOK).Inc()
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
	start := time.Now()
	wh.handlePayload(ctx)

	metrics.WriteRequestTime(ctx.Method(), ctx.Response.StatusCode()).
		Observe(Microseconds(time.Since(start)))
	metrics.WriteRequestSize(ctx.Method(), ctx.Response.StatusCode()).
		Observe(float64(ctx.Request.Header.ContentLength()))
	metrics.WriteRequestCount(ctx.Method(), ctx.Response.StatusCode()).Inc()
}

func (wh *writeHandler) handlePayload(ctx *fasthttp.RequestCtx) {
	if !ctx.IsPost() {
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}

	if ctx.Request.Header.ContentLength() > wh.maxBodySize {
		ctx.SetStatusCode(http.StatusRequestEntityTooLarge)
		return
	}

	var db string
	if param := ctx.QueryArgs().Peek("db"); param != nil {
		db = string(param)
	}
	if db == "" {
		ctx.SetStatusCode(http.StatusBadRequest)
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

	topic, err := wh.tf.Format(db)
	if err != nil {
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}

	buffer := wh.bufPool.Get()
	defer wh.bufPool.Put(buffer)

	var payloadSize int64
	parser := NewLineParser(buffer, "s")
	for {
		line, err := parser.Next(reader)
		if err == io.EOF {
			break
		}

		metrics.InfluxTotalLineCount(db).Inc()
		if err != nil {
			metrics.InfluxDroppedLineCount(db).Inc()
			continue
		}

		payloadSize = payloadSize + int64(len(line))
		metrics.InfluxLineLength(db).Observe(float64(len(line)))

		//fmt.Printf("Writing to topic %s\n", topic)
		wh.producer.Input() <- &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(line),
		}
	}

	metrics.InfluxPayloadCount(db).Inc()
	metrics.InfluxPayloadSize(db).Observe(float64(payloadSize))
	ctx.SetStatusCode(http.StatusNoContent)
}
