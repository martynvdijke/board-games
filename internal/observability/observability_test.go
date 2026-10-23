package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func testMiddleware(t *testing.T, next http.Handler) http.Handler {
	t.Helper()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return Middleware(next, nil, nil)
}

func TestMiddlewareExtractsW3CTraceContext(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	var got trace.SpanContext
	handler := testMiddleware(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanContextFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !got.IsValid() || !got.IsRemote() {
		t.Fatalf("expected a valid remote parent span context, got %v", got)
	}
	if got.TraceID().String() != traceID {
		t.Fatalf("trace id = %s, want %s", got.TraceID().String(), traceID)
	}
	if rec.Header().Get("X-Trace-ID") != traceID {
		t.Fatalf("X-Trace-ID = %q, want %q", rec.Header().Get("X-Trace-ID"), traceID)
	}
}

func TestMiddlewarePreservesRequestID(t *testing.T) {
	var requestID string
	handler := testMiddleware(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = RequestID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-abc-123")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if requestID != "req-abc-123" {
		t.Fatalf("request id = %q", requestID)
	}
}

func TestMiddlewareMintsRequestID(t *testing.T) {
	var requestID string
	handler := testMiddleware(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID = RequestID(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if len(requestID) != 24 {
		t.Fatalf("generated request id = %q, want 24 hex chars", requestID)
	}
}

func TestMiddlewareRecoversPanic(t *testing.T) {
	handler := testMiddleware(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "internal server error" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRouteLabel(t *testing.T) {
	cases := map[string]string{
		"/":                       "/",
		"/api/v1/games":           "/api/v1/games",
		"/api/v1/games/42":        "/api/v1/games/{id}",
		"/api/v1/plays/9":         "/api/v1/plays/{id}",
		"/games/7":                "/games/{id}",
		"/media/bgg/123.jpg":      "/media/bgg/{id}",
		"/static/assets/app-1.js": "/static/assets/app-1.js",
	}
	for path, want := range cases {
		if got := routeLabel(path); got != want {
			t.Errorf("routeLabel(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestRecordHelpersAreNilSafe(t *testing.T) {
	before := activeMetrics.Swap(nil)
	t.Cleanup(func() { activeMetrics.Store(before) })
	RecordBGGRequest(context.Background(), "collection", 200)
	RecordPlayCreated(context.Background(), "local")
}
