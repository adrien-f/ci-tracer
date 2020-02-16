package tracing

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	ddopentrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentracer"
	ddtrace "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func BuildTracer(tracer string) (opentracing.Tracer, func(), error) {
	if tracer == "jaeger" {
		tracer, closer, err := buildJaegerTracer()
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize Jaeger tracer: %w", err)
		}
		return tracer, func() {
			closer.Close()
		}, nil
	}
	if tracer == "datadog" {
		tracer, err := buildDatadogTracer()
		if err != nil {
			return nil, nil, fmt.Errorf("could not initialize Datadog tracer: %w", err)
		}
		return tracer, func() {
			ddtrace.Stop()
		}, nil
	}
	return nil, nil, nil
}

func buildJaegerTracer() (opentracing.Tracer, io.Closer, error) {
	cfg := jaegercfg.Configuration{
		ServiceName: "ci-tracer",
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LocalAgentHostPort: "127.0.0.1:6831",
		},
	}
	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(jaegercfg.Logger(jLogger))
	if err != nil {
		return nil, nil, nil
	}
	return tracer, closer, err
}

func buildDatadogTracer() (opentracing.Tracer, error) {
	t := ddopentrace.New()
	return t, nil
}
