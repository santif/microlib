package observability

import (
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPTracingMiddleware creates middleware for automatic HTTP request tracing.
// This middleware automatically creates spans for each HTTP request and propagates
// trace context through HTTP headers.
//
// Usage:
//
//	router := http.NewServeMux()
//	tracer := observability.NewTracer()
//
//	// Apply the middleware to your handlers
//	wrappedHandler := observability.HTTPTracingMiddleware(tracer)(yourHandler)
//	router.Handle("/api", wrappedHandler)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//
// Returns:
//   - A middleware function that can be applied to an http.Handler
func HTTPTracingMiddleware(tracer Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from request headers
			ctx := tracer.Extract(r.Context(), headerCarrier(r.Header))

			// Start a new span for this request
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethod(r.Method),
					semconv.HTTPRoute(r.URL.Path),
					semconv.HTTPURL(r.URL.String()),
					attribute.String("http.user_agent", r.UserAgent()),
					semconv.HTTPRequestContentLengthKey.Int64(r.ContentLength),
					semconv.HTTPScheme(r.URL.Scheme),
					semconv.NetHostName(r.Host),
				),
			)
			defer span.End()

			// Create a response writer wrapper to capture the status code
			wrapper := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Add the span to the request context
			r = r.WithContext(ctx)

			// Process the request
			next.ServeHTTP(wrapper, r)

			// Add response attributes to the span
			span.SetAttributes(
				semconv.HTTPStatusCodeKey.Int(wrapper.statusCode),
				attribute.Int64("http.response_size", wrapper.written),
			)

			// Mark the span as error if the status code is 4xx or 5xx
			if wrapper.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(wrapper.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// CombinedHTTPMiddleware creates middleware that combines tracing, metrics, and logging.
// This middleware applies all three observability concerns in a single middleware function.
//
// Usage:
//
//	router := http.NewServeMux()
//	tracer := observability.NewTracer()
//	metrics := observability.NewMetrics()
//	logger := observability.NewLogger()
//
//	// Apply the combined middleware to your handlers
//	wrappedHandler := observability.CombinedHTTPMiddleware(tracer, metrics, logger)(yourHandler)
//	router.Handle("/api", wrappedHandler)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//   - metrics: The Metrics instance to use for recording metrics
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A middleware function that can be applied to an http.Handler
func CombinedHTTPMiddleware(tracer Tracer, metrics Metrics, logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply middlewares in the correct order: tracing first, then metrics, then logging
		tracingMiddleware := HTTPTracingMiddleware(tracer)
		metricsMiddleware := HTTPMetricsMiddleware(metrics)

		return tracingMiddleware(metricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the current span from context
			span := trace.SpanFromContext(r.Context())

			// Log the request with trace correlation
			logger.InfoContext(r.Context(), "HTTP request started",
				NewField("method", r.Method),
				NewField("path", r.URL.Path),
				NewField("trace_id", span.SpanContext().TraceID().String()),
				NewField("span_id", span.SpanContext().SpanID().String()),
			)

			// Process the request
			next.ServeHTTP(w, r)
		})))
	}
}
