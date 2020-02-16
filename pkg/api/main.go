package api

import (
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
)

type API struct {
	logger *log.Entry
	tracer opentracing.Tracer
}
