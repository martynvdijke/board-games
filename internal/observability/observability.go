// Package observability wires structured logging, request correlation,
// OpenTelemetry traces/metrics, and panic recovery.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"boardgames/internal/config"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Providers struct {
	Shutdown func(context.Context) error
	Metrics  *Metrics
}

type Metrics struct {
	Requests metric.Int64Counter
	Duration metric.Float64Histogram
	BGG      metric.Int64Counter
	Plays    metric.Int64Counter
}

var activeMetrics atomic.Pointer[Metrics]

// RecordBGGRequest counts an attempt against the BoardGameGeek XML API.
func RecordBGGRequest(ctx context.Context, endpoint string, status int) {
	m := activeMetrics.Load()
	if m == nil || m.BGG == nil {
		return
	}
	m.BGG.Add(ctx, 1, metric.WithAttributes(
		attribute.String("bgg.endpoint", endpoint),
		attribute.String("http.status_code", strconv.Itoa(status)),
	))
}

// RecordPlayCreated counts a play logged by a person (not an import).
func RecordPlayCreated(ctx context.Context, origin string) {
	m := activeMetrics.Load()
	if m == nil || m.Plays == nil {
		return
	}
	m.Plays.Add(ctx, 1, metric.WithAttributes(attribute.String("origin", origin)))
}

type ctxKey string

const requestIDKey ctxKey = "request_id"

// Setup configures slog and optional OTLP exporters.
func Setup(ctx context.Context, cfg *config.Config) (*Providers, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	if cfg.Env == "development" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	}
	slog.SetDefault(logger)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("opentelemetry error", "error", err)
	}))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.OTELServiceName),
		semconv.ServiceVersion(cfg.Version),
		semconv.DeploymentEnvironmentName(cfg.Env),
	)

	var traceOpts []sdktrace.TracerProviderOption
	traceOpts = append(traceOpts, sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.2))))
	if cfg.OTLPEndpoint != "" {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otlp trace exporter: %w", err)
		}
		traceOpts = append(traceOpts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(traceOpts...)
	otel.SetTracerProvider(tp)

	var shutdowns []func(context.Context) error
	shutdowns = append(shutdowns, tp.Shutdown)

	var metricOpts []sdkmetric.Option
	metricOpts = append(metricOpts, sdkmetric.WithResource(res))
	if cfg.OTLPEndpoint != "" {
		mexp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otlp metric exporter: %w", err)
		}
		metricOpts = append(metricOpts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexp)))
	}
	promExp, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}
	metricOpts = append(metricOpts, sdkmetric.WithReader(promExp))
	mp := sdkmetric.NewMeterProvider(metricOpts...)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	m := &Metrics{}
	meter := mp.Meter("boardgames")
	m.Requests, _ = meter.Int64Counter("http.server.requests", metric.WithDescription("HTTP requests by route and status"))
	m.Duration, _ = meter.Float64Histogram("http.server.duration", metric.WithUnit("s"))
	m.BGG, _ = meter.Int64Counter("bgg.requests", metric.WithDescription("BoardGameGeek API requests"))
	m.Plays, _ = meter.Int64Counter("plays.created", metric.WithDescription("Locally logged plays"))
	activeMetrics.Store(m)

	return &Providers{
		Shutdown: func(ctx context.Context) error {
			var first error
			for _, fn := range shutdowns {
				if err := fn(ctx); err != nil && first == nil {
					first = err
				}
			}
			return first
		},
		Metrics: m,
	}, nil
}

func (p *Providers) MetricsHandler() http.Handler {
	if p == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
	return promhttp.Handler()
}

// NewRequestID mints a correlation identifier.
func NewRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestID returns the correlation id stored on the context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// Middleware adds request ids, traces, metrics, structured logs, and recovery.
func Middleware(next http.Handler, metrics *Metrics, trusted []*net.IPNet) http.Handler {
	tracer := otel.Tracer("http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" || len(reqID) > 64 {
			reqID = NewRequestID()
		}
		parent := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx := context.WithValue(parent, requestIDKey, reqID)
		route := routeLabel(r.URL.Path)
		ctx, span := tracer.Start(ctx, r.Method+" "+route,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.String("url.path", r.URL.Path),
			))
		defer span.End()
		if span.SpanContext().IsValid() {
			w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
		}
		r = r.WithContext(ctx)
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "request_id", reqID, "trace_id", span.SpanContext().TraceID().String(), "panic", rec, "stack", string(debug.Stack()))
				span.RecordError(fmt.Errorf("panic: %v", rec))
				span.SetStatus(codes.Error, "panic")
				http.Error(rw, "internal server error", http.StatusInternalServerError)
			}
			dur := time.Since(start)
			span.SetAttributes(attribute.Int("http.response.status_code", rw.status))
			if rw.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rw.status))
			}
			if metrics != nil {
				metrics.Requests.Add(ctx, 1, metric.WithAttributes(
					attribute.String("http.route", route),
					attribute.String("http.status", statusClass(rw.status)),
					attribute.String("http.method", r.Method),
				))
				metrics.Duration.Record(ctx, dur.Seconds(), metric.WithAttributes(attribute.String("http.route", route)))
			}
			slog.Info("http request", "request_id", reqID, "trace_id", span.SpanContext().TraceID().String(),
				"span_id", span.SpanContext().SpanID().String(), "method", r.Method, "path", r.URL.Path,
				"route", route, "status", rw.status, "duration_ms", dur.Milliseconds(), "remote", realIP(r, trusted))
		}()
		next.ServeHTTP(rw, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush supports streaming responses.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func routeLabel(path string) string {
	if strings.HasPrefix(path, "/api/v1/games/") {
		return "/api/v1/games/{id}"
	}
	if strings.HasPrefix(path, "/api/v1/plays/") {
		return "/api/v1/plays/{id}"
	}
	if strings.HasPrefix(path, "/games/") {
		return "/games/{id}"
	}
	if strings.HasPrefix(path, "/media/bgg/") {
		return "/media/bgg/{id}"
	}
	return path
}

func statusClass(code int) string {
	return fmt.Sprintf("%dxx", code/100)
}

// RealIP returns the client address, honouring X-Forwarded-For only from a
// trusted proxy range.
func RealIP(r *http.Request, trusted []*net.IPNet) string { return realIP(r, trusted) }

func realIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !isTrusted(ip, trusted) {
		return host
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	parts := strings.Split(xff, ",")
	candidate := strings.TrimSpace(parts[0])
	if net.ParseIP(candidate) != nil {
		return candidate
	}
	return host
}

func isTrusted(ip net.IP, trusted []*net.IPNet) bool {
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies turns configuration strings into networks.
func ParseTrustedProxies(values []string) []*net.IPNet {
	var out []*net.IPNet
	for _, v := range values {
		if _, n, err := net.ParseCIDR(v); err == nil {
			out = append(out, n)
			continue
		}
		if ip := net.ParseIP(v); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return out
}
