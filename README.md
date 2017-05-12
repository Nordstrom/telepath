telepath
--------

An endpoint to receive telemetry destined for Kafka.

## example

Build and run Telepath.

```
make
bin/telepath -broker=localhost:9092
```

Post a metric in Influx line-protocol.

```
curl -i -XPOST http://localhost:4567/write -d 'foo,host=localhost value=1 1468928660000000000
'
```

## notes

Using [dep](https://github.com/golang/dep) for vendoring.
