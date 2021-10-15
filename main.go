package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	discovery "github.com/gkarthiks/k8s-discovery"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"net/http"
)

func ping(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "pong\n")
}

func initTracer() *sdktrace.TracerProvider {
	exporter, err := stdout.New(stdout.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String("Service-2"))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

func main() {
	tp := initTracer()

	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.Handle("/ping",
		otelhttp.NewHandler(http.HandlerFunc(ping),
			"ping"))
	http.Handle("/pods",
		otelhttp.NewHandler(http.HandlerFunc(listPods),
			"List Pods"))

	http.ListenAndServe(":8091", nil)
}

func listPods(w http.ResponseWriter, req *http.Request) {
	k8s, _ := discovery.NewK8s()

	podList, _ := k8s.Clientset.CoreV1().Pods("").List(context.Background(), v1.ListOptions{})
	var pods []string
	for _, pod := range podList.Items {
		pods = append(pods, pod.Name)
	}

	buffer := &bytes.Buffer{}
	gob.NewEncoder(buffer).Encode(pods)
	byteSlice := buffer.Bytes()

	w.WriteHeader(200)
	w.Write(byteSlice)
}
