package handler

import (
	"net/http"

	"github.com/flowchartsman/swaggerui"
	"github.com/pratham-singh/ticket-booking/internal/apidocs"
)

func RegisterDocs(r interface {
	Get(pattern string, h http.HandlerFunc)
	Mount(pattern string, h http.Handler)
}) {
	spec := apidocs.OpenAPI
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(spec)
	})
	r.Mount("/docs", http.StripPrefix("/docs", swaggerui.Handler(spec)))
}
