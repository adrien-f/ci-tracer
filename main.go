package main

import (
	"encoding/json"
	"net/http"
	"time"

	"git.manomano.tech/sre/ci-tracer/pkg/api"
	"git.manomano.tech/sre/ci-tracer/pkg/tracing"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app           = kingpin.New("ci-tracer", "Program to trace CI pipelines")
	listenAddress = app.Flag("listen-address", "server listening address").Default("127.0.0.1:3000").String()
	tracerImpl    = app.Flag("tracer", "tracer implementation to use").Default("jaeger").Enum("jaeger", "datadog")
)

func main() {
	kingpin.Parse()

	logger := log.New()

	tracer, closer, err := tracing.BuildTracer(*tracerImpl)
	if err != nil {
		log.Fatal(err)
	}
	opentracing.SetGlobalTracer(tracer)
	defer closer()

	router := buildRouter(logger, tracer)
	http.Handle("/", router)

	srv := &http.Server{
		Handler:      router,
		Addr:         *listenAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func buildRouter(logger *log.Logger, tracer opentracing.Tracer) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	g := api.NewGitlabAPI(logger, tracer)
	g.Register(r)
	return r
}
