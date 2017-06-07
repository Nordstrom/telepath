telepath
--------

[![Build Status](https://travis-ci.org/Nordstrom/telepath.svg?branch=master)](https://travis-ci.org/Nordstrom/telepath)

An HTTP endpoint to receive [Influx](https://github.com/influxdata/influxdb) line-protocol metrics destined for [Kafka](http://kafka.apache.org/).

_Note: not ready for production!_

## example

Build and run Telepath.

```
make
bin/telepath -broker=localhost:9092
```

Post a metric in Influx line-protocol.

```
curl -i -XPOST http://localhost:8089/write -d 'foo,host=localhost value=1 1468928660000000000'
```

## notes

Using [dep](https://github.com/golang/dep) for vendoring.
