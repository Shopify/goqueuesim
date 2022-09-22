package metrics

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/rs/zerolog/log"
)

const (
	defaultStatsdAddr = "127.0.0.1:8125" // TODO: Move this into cli flag.
	statsdNamespace   = "checkout_queue_simulator."
	statsdScope       = "default"
)

var Client statsd.ClientInterface
var runtimeGlobalTags = make([]string, 0)

func init() {
	c, err := statsd.New(defaultStatsdAddr)
	if err != nil {
		Client = &statsd.NoOpClient{}
		log.Info().Msg("failed connecting to datadog agent => metrics will noop")
	} else {
		Client = c
		c.Namespace = statsdNamespace
		c.Tags = []string{fmt.Sprintf("scope:%s", statsdScope)}
		log.Info().Msg("successfully connected to datadog agent")
	}
}

func AddGlobalTags(tags []string) {
	runtimeGlobalTags = append(runtimeGlobalTags, tags...)
}

func Count(name string, value int64, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Count(name, value, tags, 1.0 /* rate */)
}

func Decr(name string, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Decr(name, tags, 1.0 /* rate */)
}

func Distribution(name string, value float64, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Distribution(name, value, tags, 1.0 /* rate */)
}

func Gauge(name string, value float64, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Gauge(name, value, tags, 1.0 /* rate */)
}

func Histogram(name string, value float64, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Histogram(name, value, tags, 1.0 /* rate */)
}

func Incr(name string, tags []string) error {
	tags = append(runtimeGlobalTags, tags...)
	return Client.Incr(name, tags, 1.0 /* rate */)
}

func BenchmarkMethod(startTime time.Time, methodName string, tags []string) {
	elapsed := time.Since(startTime)
	metricName := fmt.Sprintf("%s.elapsed_ns", methodName)
	Distribution(metricName, float64(elapsed.Nanoseconds()), tags)
}
