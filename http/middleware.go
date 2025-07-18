package http

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/santif/microlib/observability"
)

// Middleware is a function that wraps an http.Handler and returns a new http.Handler
type Middleware func(http.Handler) http.Handler

// MiddlewareChain represents a chain of middleware that can be applied to an http.Handler
type MiddlewareChain []Middleware

// Apply applies the middleware chain to the given handler in the order they were added
func (mc MiddlewareChain) Apply(handler http.Handler) http.Handler {
	// Apply middleware in reverse order so that the first middleware in the chain
	// is the outermost wrapper (i.e., it gets executed first on the way in and last on the way out)
	for i := len(mc) - 1; i >= 0; i-- {
		handler = mc[i](handler)
	}
	return handler
}

// responseWriterWrapper is a wrapper around http.ResponseWriter that captures the status code and response size
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// WriteHeader captures the status code before calling the wrapped ResponseWriter's WriteHeader
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the number of bytes written before calling the wrapped ResponseWriter's Write
func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

// Flush implements the http.Flusher interface if the wrapped ResponseWriter implements it
func (w *responseWriterWrapper) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface if the wrapped ResponseWriter implements it
func (w *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// LoggingMiddleware creates middleware that logs HTTP requests with trace correlation
func LoggingMiddleware(logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture the status code and response size
			wrapper := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Log the request with trace correlation
			logger.InfoContext(r.Context(), "HTTP request started",
				observability.NewField("method", r.Method),
				observability.NewField("path", r.URL.Path),
				observability.NewField("remote_addr", r.RemoteAddr),
				observability.NewField("user_agent", r.UserAgent()),
			)

			// Process the request
			next.ServeHTTP(wrapper, r)

			// Calculate duration
			duration := time.Since(start)

			// Log the response
			if wrapper.statusCode >= 400 {
				logger.ErrorContext(r.Context(), "HTTP request completed", fmt.Errorf("status code %d", wrapper.statusCode),
					observability.NewField("method", r.Method),
					observability.NewField("path", r.URL.Path),
					observability.NewField("status", wrapper.statusCode),
					observability.NewField("duration_ms", duration.Milliseconds()),
					observability.NewField("size", wrapper.written),
				)
			} else {
				logger.InfoContext(r.Context(), "HTTP request completed",
					observability.NewField("method", r.Method),
					observability.NewField("path", r.URL.Path),
					observability.NewField("status", wrapper.statusCode),
					observability.NewField("duration_ms", duration.Milliseconds()),
					observability.NewField("size", wrapper.written),
				)
			}
		})
	}
}

// MetricsMiddleware creates middleware for automatic HTTP metrics collection
func MetricsMiddleware(metrics observability.Metrics) Middleware {
	return observability.HTTPMetricsMiddleware(metrics)
}

// TracingMiddleware creates middleware for automatic HTTP request tracing
func TracingMiddleware(tracer observability.Tracer) Middleware {
	return observability.HTTPTracingMiddleware(tracer)
}

// RecoveryMiddleware creates middleware that recovers from panics and logs the error
func RecoveryMiddleware(logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Convert the recovered value to an error
					var err error
					switch v := rec.(type) {
					case error:
						err = v
					case string:
						err = fmt.Errorf("%s", v)
					default:
						err = fmt.Errorf("%v", v)
					}

					// Log the panic with context
					logger.ErrorContext(r.Context(), "HTTP handler panic recovered", err,
						observability.NewField("method", r.Method),
						observability.NewField("path", r.URL.Path),
						observability.NewField("remote_addr", r.RemoteAddr),
					)

					// Return a 500 Internal Server Error
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware creates middleware that adds CORS headers to responses
func CORSMiddleware(config CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			if config.AllowOrigins != "" {
				w.Header().Set("Access-Control-Allow-Origin", config.AllowOrigins)
			}

			if config.AllowMethods != "" {
				w.Header().Set("Access-Control-Allow-Methods", config.AllowMethods)
			}

			if config.AllowHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", config.AllowHeaders)
			}

			if config.ExposeHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", config.ExposeHeaders)
			}

			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware creates middleware that adds security headers to responses
func SecurityHeadersMiddleware(config SecurityHeadersConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set security headers
			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			if config.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.XFrameOptions)
			}

			if config.XContentTypeOptions {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			if config.StrictTransportSecurity != "" {
				w.Header().Set("Strict-Transport-Security", config.StrictTransportSecurity)
			}

			if config.XSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CombinedMiddleware creates middleware that combines tracing, metrics, and logging
func CombinedMiddleware(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		// Apply middlewares in the correct order: tracing first, then metrics, then logging
		tracingMiddleware := TracingMiddleware(tracer)
		metricsMiddleware := MetricsMiddleware(metrics)
		loggingMiddleware := LoggingMiddleware(logger)

		return tracingMiddleware(metricsMiddleware(loggingMiddleware(next)))
	}
}

// DefaultMiddleware creates a default middleware stack with all standard middleware
func DefaultMiddleware(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) MiddlewareChain {
	return MiddlewareChain{
		RecoveryMiddleware(logger),
		TracingMiddleware(tracer),
		MetricsMiddleware(metrics),
		LoggingMiddleware(logger),
		SecurityHeadersMiddleware(DefaultSecurityHeadersConfig()),
	}
}

// HealthMiddleware creates middleware that skips other middleware for health check endpoints
func HealthMiddleware(healthPaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request path is a health check endpoint
			for _, path := range healthPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					// Skip other middleware for health check endpoints
					next.ServeHTTP(w, r)
					return
				}
			}

			// Continue with the middleware chain for non-health check endpoints
			next.ServeHTTP(w, r)
		})
	}
}
