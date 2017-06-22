package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func Microseconds(d time.Duration) float64 {
	return float64(d.Nanoseconds() / time.Microsecond.Nanoseconds())
}

type prometheusMetrics struct {
	sync.Mutex
	handler fasthttp.RequestHandler

	pingRequestCount  *prometheus.CounterVec
	queryRequestCount *prometheus.CounterVec
	writeRequestCount *prometheus.CounterVec
	writeRequestTime  *prometheus.SummaryVec
	writeRequestSize  *prometheus.SummaryVec

	influxPayloadCount     *prometheus.CounterVec
	influxPayloadSize      *prometheus.SummaryVec
	influxTotalLineCount   *prometheus.CounterVec
	influxDroppedLineCount *prometheus.CounterVec
	influxLineLength       *prometheus.SummaryVec

	kafkaProducerSuccessCount *prometheus.CounterVec
	kafkaProducerErrorCount   *prometheus.CounterVec
}

var register sync.Once

var metrics *prometheusMetrics

func (m *prometheusMetrics) Handle(ctx *fasthttp.RequestCtx) {
	m.handler(ctx)
}

func (m *prometheusMetrics) QueryRequestCount(verb []byte, status int) prometheus.Counter {
	return m.queryRequestCount.WithLabelValues(string(verb), strconv.Itoa(status))
}

func (m *prometheusMetrics) PingRequestCount(verb []byte, status int) prometheus.Counter {
	return m.pingRequestCount.WithLabelValues(string(verb), strconv.Itoa(status))
}

func (m *prometheusMetrics) WriteRequestCount(verb []byte, status int) prometheus.Counter {
	return m.writeRequestCount.WithLabelValues(string(verb), strconv.Itoa(status))
}

func (m *prometheusMetrics) WriteRequestTime(verb []byte, status int) prometheus.Summary {
	return m.writeRequestTime.WithLabelValues(string(verb), strconv.Itoa(status))
}

func (m *prometheusMetrics) WriteRequestSize(verb []byte, status int) prometheus.Summary {
	return m.writeRequestSize.WithLabelValues(string(verb), strconv.Itoa(status))
}

func (m *prometheusMetrics) InfluxPayloadCount(db string) prometheus.Counter {
	return m.influxPayloadCount.WithLabelValues(db)
}

func (m *prometheusMetrics) InfluxPayloadSize(db string) prometheus.Summary {
	return m.influxPayloadSize.WithLabelValues(db)
}

func (m *prometheusMetrics) InfluxTotalLineCount(db string) prometheus.Counter {
	return m.influxTotalLineCount.WithLabelValues(db)
}

func (m *prometheusMetrics) InfluxDroppedLineCount(db string) prometheus.Counter {
	return m.influxDroppedLineCount.WithLabelValues(db)
}

func (m *prometheusMetrics) InfluxLineLength(db string) prometheus.Summary {
	return m.influxLineLength.WithLabelValues(db)
}

func (m *prometheusMetrics) KafkaProducerSuccessCount(topic string) prometheus.Counter {
	return m.kafkaProducerSuccessCount.WithLabelValues(topic)
}

func (m *prometheusMetrics) KafkaProducerErrorCount(topic string) prometheus.Counter {
	return m.kafkaProducerErrorCount.WithLabelValues(topic)
}

func init() {
	metrics = &prometheusMetrics{
		handler: fasthttpadaptor.NewFastHTTPHandler(prometheus.Handler()),
		pingRequestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "ping",
			Name:      "requests_total",
			Help:      "Count of requests against the /ping endpoint",
		}, []string{"verb", "status"}),

		queryRequestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "query",
			Name:      "requests_total",
			Help:      "Count of requests against the /query endpoint",
		}, []string{"verb", "status"}),

		writeRequestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "write",
			Name:      "requests_total",
			Help:      "Count of requests against the /write endpoint",
		}, []string{"verb", "status"}),

		writeRequestTime: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "telepath",
			Subsystem: "write",
			Name:      "request_duration_microseconds",
			Help:      "Latency of requests against the /write endpoint in microseconds",
		}, []string{"verb", "status"}),

		writeRequestSize: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "telepath",
			Subsystem: "write",
			Name:      "request_size_bytes",
			Help:      "Size of requests to the /write endpoint in bytes",
		}, []string{"verb", "status"}),

		influxPayloadCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "influx",
			Name:      "payloads_total",
			Help:      "Count of Influx metric payloads",
		}, []string{"db"}),

		influxPayloadSize: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "telepath",
			Subsystem: "influx",
			Name:      "payload_size_bytes",
			Help:      "Size of Influx metrics payloads in bytes",
		}, []string{"db"}),

		influxTotalLineCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "influx",
			Name:      "lines_total",
			Help:      "Count of Influx metric lines",
		}, []string{"db"}),

		influxDroppedLineCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "influx",
			Name:      "dropped_lines_total",
			Help:      "Count of invalid or unparsable Influx metric lines",
		}, []string{"db"}),

		influxLineLength: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "telepath",
			Subsystem: "influx",
			Name:      "line_length_bytes",
			Help:      "Size of Influx metric lines in bytes",
		}, []string{"db"}),

		kafkaProducerSuccessCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "kafka_producer",
			Name:      "successes_total",
			Help:      "Count of successes returned from Kafka producer",
		}, []string{"topic"}),

		kafkaProducerErrorCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "telepath",
			Subsystem: "kafka_producer",
			Name:      "errors_total",
			Help:      "Count of errors returned from Kafka producer",
		}, []string{"topic"}),
	}

	register.Do(func() {
		prometheus.MustRegister(metrics.pingRequestCount)
		prometheus.MustRegister(metrics.queryRequestCount)
		prometheus.MustRegister(metrics.writeRequestCount)
		prometheus.MustRegister(metrics.writeRequestTime)
		prometheus.MustRegister(metrics.writeRequestSize)

		prometheus.MustRegister(metrics.influxPayloadCount)
		prometheus.MustRegister(metrics.influxPayloadSize)
		prometheus.MustRegister(metrics.influxTotalLineCount)
		prometheus.MustRegister(metrics.influxDroppedLineCount)
		prometheus.MustRegister(metrics.influxLineLength)

		prometheus.MustRegister(metrics.kafkaProducerSuccessCount)
		prometheus.MustRegister(metrics.kafkaProducerErrorCount)
	})
}
