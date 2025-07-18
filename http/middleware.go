package http

import (
	"fmt"
	"net/http"
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
func (w *responseWriterWrapper) Hijack() (interface{}, interface{}, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// RecoveryMiddleware creates middleware that recovers from panics and logs the error
func RecoveryMiddleware(logger interface {
	Error(msg string, err error, fields ...interface{})
}) Middleware {
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

					// Log the panic
					logger.Error("HTTP handler panic recovered", err)

					// Return a 500 Internal Server Error
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
