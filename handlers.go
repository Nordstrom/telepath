package main

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
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
const BytePoolCount = 128

type writeHandler struct {
	bytePool    *bpool.BytePool
	maxBodySize int
	producer    sarama.AsyncProducer
	tt          *topicTemplate
}

type writeConfig struct {
	maxBodySize   int
	maxLineSize   int
	maxChunkSize  int
	bytePoolCount int
	topicTemplate string
}

func NewWriteHandler(producer sarama.AsyncProducer, config writeConfig) (*writeHandler, error) {
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
	bytePoolCount := config.bytePoolCount
	if bytePoolCount < 1 {
		bytePoolCount = BytePoolCount
	}

	template, err := NewTopicTemplate(config.topicTemplate)
	if err != nil {
		return nil, err
	}

	return &writeHandler{
		maxBodySize: maxBodySize,
		bytePool:    bpool.NewBytePool(bytePoolCount, maxChunkSize),
		producer:    producer,
		tt:          template,
	}, nil
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

	contentLength := ctx.Request.Header.ContentLength()
	if contentLength > wh.maxBodySize {
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

	precision := "ns"
	if param := ctx.QueryArgs().Peek("precision"); param != nil {
		precision = string(param)
	}

	contentEncoding := "text/plain"
	if header := ctx.Request.Header.Peek("Content-Encoding"); header != nil {
		contentEncoding = string(header)
	}

	var reader io.Reader
	if string(contentEncoding) != "gzip" {
		reader = bytes.NewReader(ctx.Request.Body())
	} else {
		body, err := ctx.Request.BodyGunzip()
		if err != nil {
			log.WithError(err).WithFields(
				log.Fields{db: db}).Error("Couldn't gunzip the payload.")
			ctx.SetStatusCode(http.StatusBadRequest)
			return
		}

		reader = bytes.NewReader(body)
	}

	topic, err := wh.tt.Execute(db)
	if err != nil {
		log.WithError(err).WithFields(
			log.Fields{db: db}).Error("Couldn't build a topic.")
		ctx.SetStatusCode(http.StatusBadRequest)
		return
	}

	log.WithFields(log.Fields{
		"db":               db,
		"precision":        precision,
		"topic":            topic,
		"content-length":   contentLength,
		"content-encoding": contentEncoding,
	}).Debugf("Handling payload for '%s' database.", db)

	buffer := wh.bytePool.Get()
	defer wh.bytePool.Put(buffer)

	var payloadSize int64
	parser := NewLineParser(buffer, precision)
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

		wh.producer.Input() <- &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(line),
		}
	}

	ctx.SetStatusCode(http.StatusNoContent)
	metrics.InfluxPayloadCount(db).Inc()
	metrics.InfluxPayloadSize(db).Observe(float64(payloadSize))
}
