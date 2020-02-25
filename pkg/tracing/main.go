package tracing

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	ddopentrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	ddtrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

// BuildTracer tries to create a tracer based on input
func BuildTracer(tracer string) (opentracing.Tracer, func(), error) {
	switch tracer {
	case "jaeger":
		tracer, closer, err := buildJaegerTracer()
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize Jaeger tracer: %w", err)
		}
		return tracer, func() {
			closer.Close()
		}, nil
	case "datadog":
		tracer, err := buildDatadogTracer()
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize Datadog tracer: %w", err)
		}
		return tracer, func() {
			ddtrace.Stop()
		}, nil
	default:
		return nil, nil, fmt.Errorf("unknown tracer: %s", tracer)
	}
}

func buildJaegerTracer() (opentracing.Tracer, io.Closer, error) {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		return nil, nil, err
	}
	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(jaegercfg.Logger(jLogger))
	if err != nil {
		return nil, nil, err
	}
	return tracer, closer, err
}

func buildDatadogTracer() (opentracing.Tracer, error) {
	t := ddopentrace.New()
	return t, nil
}
