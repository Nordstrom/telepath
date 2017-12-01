telepath
--------

[![Build Status](https://travis-ci.org/Nordstrom/telepath.svg?branch=master)](https://travis-ci.org/Nordstrom/telepath)

An HTTP endpoint to receive [Influx](https://github.com/influxdata/influxdb) line-protocol metrics destined for [Kafka](http://kafka.apache.org/).

## example

Build and run Telepath.

```
make
bin/telepath -kafka.broker=localhost:9092
```

Post a metric in Influx line-protocol.

```
curl -i -XPOST http://localhost:8089/write -d 'foo,host=localhost value=1 1468928660000000000'
```

Additionally, this project contains a [docker-compose](https://docs.docker.com/compose) file that uses [Telegraf](http://github.com/influxdata/telegraf) and [Jolokia](https://jolokia.org) to send Kafka's own metrics into a Kafka topic.

```
docker-compose build
docker-compose up
```

## notes

- We're currently using [dep](https://github.com/golang/dep) for vendoring.
- The default Kafka Producer behavior is based on [Sarama version](https://godoc.org/github.com/Shopify/sarama#pkg-variables) ` V0_10_0_0`
