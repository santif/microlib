package grpc

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/santif/microlib/observability"
	"github.com/santif/microlib/security"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// LoggingUnaryServerInterceptor creates a gRPC server interceptor for logging unary RPCs.
// This interceptor automatically logs the start and completion of each gRPC request.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.LoggingUnaryServerInterceptor(logger),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func LoggingUnaryServerInterceptor(logger observability.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Log the request
		logger.InfoContext(ctx, "gRPC request started",
			observability.NewField("method", info.FullMethod),
			observability.NewField("request_type", fmt.Sprintf("%T", req)),
		)

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(start)

		// Log the response
		if err != nil {
			s, _ := status.FromError(err)
			logger.ErrorContext(ctx, "gRPC request failed",
				err,
				observability.NewField("method", info.FullMethod),
				observability.NewField("status_code", s.Code().String()),
				observability.NewField("duration_ms", duration.Milliseconds()),
			)
		} else {
			logger.InfoContext(ctx, "gRPC request completed",
				observability.NewField("method", info.FullMethod),
				observability.NewField("duration_ms", duration.Milliseconds()),
				observability.NewField("response_type", fmt.Sprintf("%T", resp)),
			)
		}

		return resp, err
	}
}

// LoggingStreamServerInterceptor creates a gRPC server interceptor for logging streaming RPCs.
// This interceptor automatically logs the start and completion of each gRPC streaming request.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.LoggingStreamServerInterceptor(logger),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func LoggingStreamServerInterceptor(logger observability.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()

		// Log the stream start
		logger.InfoContext(ctx, "gRPC stream started",
			observability.NewField("method", info.FullMethod),
			observability.NewField("is_client_stream", info.IsClientStream),
			observability.NewField("is_server_stream", info.IsServerStream),
		)

		// Process the stream
		err := handler(srv, ss)

		// Calculate duration
		duration := time.Since(start)

		// Log the stream completion
		if err != nil {
			s, _ := status.FromError(err)
			logger.ErrorContext(ctx, "gRPC stream failed",
				err,
				observability.NewField("method", info.FullMethod),
				observability.NewField("status_code", s.Code().String()),
				observability.NewField("duration_ms", duration.Milliseconds()),
			)
		} else {
			logger.InfoContext(ctx, "gRPC stream completed",
				observability.NewField("method", info.FullMethod),
				observability.NewField("duration_ms", duration.Milliseconds()),
			)
		}

		return err
	}
}

// MetricsUnaryServerInterceptor creates a gRPC server interceptor for collecting metrics for unary RPCs.
// This interceptor automatically collects metrics for each gRPC request.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.MetricsUnaryServerInterceptor(metrics),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - metrics: The Metrics instance to use for collecting metrics
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func MetricsUnaryServerInterceptor(metrics observability.Metrics) grpc.UnaryServerInterceptor {
	// Create metrics
	requestsCounter := metrics.Counter(
		"grpc_server_requests_total",
		"Total number of gRPC requests received",
		"method", "status",
	)

	requestDuration := metrics.Histogram(
		"grpc_server_request_duration_seconds",
		"Duration of gRPC requests in seconds",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		"method", "status",
	)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get status code
		statusCode := grpccodes.OK
		if err != nil {
			s, _ := status.FromError(err)
			statusCode = s.Code()
		}

		// Record metrics
		labels := map[string]string{
			"method": info.FullMethod,
			"status": statusCode.String(),
		}

		requestsCounter.WithLabels(labels).Inc()
		requestDuration.WithLabels(labels).Observe(duration)

		return resp, err
	}
}

// MetricsStreamServerInterceptor creates a gRPC server interceptor for collecting metrics for streaming RPCs.
// This interceptor automatically collects metrics for each gRPC streaming request.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.MetricsStreamServerInterceptor(metrics),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - metrics: The Metrics instance to use for collecting metrics
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func MetricsStreamServerInterceptor(metrics observability.Metrics) grpc.StreamServerInterceptor {
	// Create metrics
	streamsCounter := metrics.Counter(
		"grpc_server_streams_total",
		"Total number of gRPC streams started",
		"method", "status", "stream_type",
	)

	streamDuration := metrics.Histogram(
		"grpc_server_stream_duration_seconds",
		"Duration of gRPC streams in seconds",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		"method", "status", "stream_type",
	)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// Process the stream
		err := handler(srv, ss)

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get status code
		statusCode := grpccodes.OK
		if err != nil {
			s, _ := status.FromError(err)
			statusCode = s.Code()
		}

		// Determine stream type
		streamType := "unary"
		if info.IsClientStream && info.IsServerStream {
			streamType = "bidi"
		} else if info.IsClientStream {
			streamType = "client"
		} else if info.IsServerStream {
			streamType = "server"
		}

		// Record metrics
		labels := map[string]string{
			"method":      info.FullMethod,
			"status":      statusCode.String(),
			"stream_type": streamType,
		}

		streamsCounter.WithLabels(labels).Inc()
		streamDuration.WithLabels(labels).Observe(duration)

		return err
	}
}

// RecoveryUnaryServerInterceptor creates a gRPC server interceptor for recovering from panics in unary RPCs.
// This interceptor automatically recovers from panics and logs the error.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.RecoveryUnaryServerInterceptor(logger),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func RecoveryUnaryServerInterceptor(logger observability.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic
				logger.ErrorContext(ctx, "gRPC panic recovered",
					fmt.Errorf("panic: %v", r),
					observability.NewField("method", info.FullMethod),
					observability.NewField("stack", string(debug.Stack())),
				)

				// Return an error to the client
				err = status.Errorf(grpccodes.Internal, "Internal server error")
			}
		}()

		// Process the request
		return handler(ctx, req)
	}
}

// RecoveryStreamServerInterceptor creates a gRPC server interceptor for recovering from panics in streaming RPCs.
// This interceptor automatically recovers from panics and logs the error.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.RecoveryStreamServerInterceptor(logger),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func RecoveryStreamServerInterceptor(logger observability.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic
				logger.ErrorContext(ss.Context(), "gRPC stream panic recovered",
					fmt.Errorf("panic: %v", r),
					observability.NewField("method", info.FullMethod),
					observability.NewField("stack", string(debug.Stack())),
				)

				// Return an error to the client
				err = status.Errorf(grpccodes.Internal, "Internal server error")
			}
		}()

		// Process the stream
		return handler(srv, ss)
	}
}

// AuthUnaryServerInterceptor creates a gRPC server interceptor for JWT authentication in unary RPCs.
// This interceptor automatically validates JWT tokens and injects claims into the context.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.AuthUnaryServerInterceptor(authenticator, []string{"/health"}),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - authenticator: The Authenticator instance to use for token validation
//   - bypassPaths: Optional list of paths to bypass authentication
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func AuthUnaryServerInterceptor(authenticator security.Authenticator, bypassPaths []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if the path should bypass authentication
		for _, path := range bypassPaths {
			if info.FullMethod == path || (path != "/" && len(info.FullMethod) >= len(path) && info.FullMethod[:len(path)] == path) {
				// Skip authentication for this path
				return handler(ctx, req)
			}
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(grpccodes.Unauthenticated, "Missing metadata")
		}

		// Get authorization header
		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Errorf(grpccodes.Unauthenticated, "Missing authorization header")
		}

		// Extract token from authorization header
		token := authHeader[0]
		if len(token) < 8 || token[:7] != "Bearer " {
			return nil, status.Errorf(grpccodes.Unauthenticated, "Invalid authorization format")
		}
		token = token[7:] // Remove "Bearer " prefix

		// Validate token
		claims, err := authenticator.ValidateToken(ctx, token)
		if err != nil {
			return nil, status.Errorf(grpccodes.Unauthenticated, "Invalid token: %v", err)
		}

		// Add claims to context
		ctx = context.WithValue(ctx, security.ClaimsContextKey, claims)

		// Add custom claims to context if specified
		for k, v := range claims.Custom {
			ctx = context.WithValue(ctx, security.ClaimContextKey(k), v)
		}

		// Process the request with the authenticated context
		return handler(ctx, req)
	}
}

// AuthStreamServerInterceptor creates a gRPC server interceptor for JWT authentication in streaming RPCs.
// This interceptor automatically validates JWT tokens and injects claims into the context.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.AuthStreamServerInterceptor(authenticator, []string{"/health"}),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - authenticator: The Authenticator instance to use for token validation
//   - bypassPaths: Optional list of paths to bypass authentication
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func AuthStreamServerInterceptor(authenticator security.Authenticator, bypassPaths []string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Check if the path should bypass authentication
		for _, path := range bypassPaths {
			if info.FullMethod == path || (path != "/" && len(info.FullMethod) >= len(path) && info.FullMethod[:len(path)] == path) {
				// Skip authentication for this path
				return handler(srv, ss)
			}
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(grpccodes.Unauthenticated, "Missing metadata")
		}

		// Get authorization header
		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return status.Errorf(grpccodes.Unauthenticated, "Missing authorization header")
		}

		// Extract token from authorization header
		token := authHeader[0]
		if len(token) < 8 || token[:7] != "Bearer " {
			return status.Errorf(grpccodes.Unauthenticated, "Invalid authorization format")
		}
		token = token[7:] // Remove "Bearer " prefix

		// Validate token
		claims, err := authenticator.ValidateToken(ctx, token)
		if err != nil {
			return status.Errorf(grpccodes.Unauthenticated, "Invalid token: %v", err)
		}

		// Add claims to context
		ctx = context.WithValue(ctx, security.ClaimsContextKey, claims)

		// Add custom claims to context if specified
		for k, v := range claims.Custom {
			ctx = context.WithValue(ctx, security.ClaimContextKey(k), v)
		}

		// Create a wrapped server stream with the authenticated context
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Process the stream with the authenticated context
		return handler(srv, wrappedStream)
	}
}

// RequireScopeUnaryInterceptor creates a gRPC server interceptor that requires a specific scope
func RequireScopeUnaryInterceptor(scope string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get claims from context
		claims, ok := security.GetClaimsFromContext(ctx)
		if !ok {
			return nil, status.Errorf(grpccodes.Unauthenticated, "No authentication claims found")
		}

		// Check if the user has the required scope
		scopes, ok := claims.Custom["scope"].(string)
		if !ok {
			return nil, status.Errorf(grpccodes.PermissionDenied, "No scopes found in token")
		}

		// Check if the scope is in the list
		scopeList := security.ParseScopeString(scopes)
		if !scopeList[scope] {
			return nil, status.Errorf(grpccodes.PermissionDenied, "Missing required scope: %s", scope)
		}

		// Call the next handler
		return handler(ctx, req)
	}
}

// RequireScopeStreamInterceptor creates a gRPC server interceptor that requires a specific scope
func RequireScopeStreamInterceptor(scope string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Get claims from context
		ctx := ss.Context()
		claims, ok := security.GetClaimsFromContext(ctx)
		if !ok {
			return status.Errorf(grpccodes.Unauthenticated, "No authentication claims found")
		}

		// Check if the user has the required scope
		scopes, ok := claims.Custom["scope"].(string)
		if !ok {
			return status.Errorf(grpccodes.PermissionDenied, "No scopes found in token")
		}

		// Check if the scope is in the list
		scopeList := security.ParseScopeString(scopes)
		if !scopeList[scope] {
			return status.Errorf(grpccodes.PermissionDenied, "Missing required scope: %s", scope)
		}

		// Call the next handler
		return handler(srv, ss)
	}
}

// CombinedObservabilityUnaryServerInterceptor creates a gRPC server interceptor that combines tracing, metrics, logging, and recovery.
// This interceptor applies all observability concerns in a single interceptor function.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.CombinedObservabilityUnaryServerInterceptor(tracer, metrics, logger)),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//   - metrics: The Metrics instance to use for recording metrics
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func CombinedObservabilityUnaryServerInterceptor(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) grpc.UnaryServerInterceptor {
	// Create the individual interceptors
	tracingInterceptor := UnaryServerTracingInterceptor(tracer)
	metricsInterceptor := MetricsUnaryServerInterceptor(metrics)
	recoveryInterceptor := RecoveryUnaryServerInterceptor(logger)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Apply the recovery interceptor first to catch panics in other interceptors
		return recoveryInterceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			// Apply the tracing interceptor
			return tracingInterceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
				// Get the current span from context
				span := trace.SpanFromContext(ctx)

				// Log the request with trace correlation
				logger.InfoContext(ctx, "gRPC request started",
					observability.NewField("method", info.FullMethod),
					observability.NewField("trace_id", span.SpanContext().TraceID().String()),
					observability.NewField("span_id", span.SpanContext().SpanID().String()),
				)

				start := time.Now()

				// Apply the metrics interceptor
				resp, err := metricsInterceptor(ctx, req, info, handler)

				// Calculate duration
				duration := time.Since(start)

				// Log the response
				if err != nil {
					s, _ := status.FromError(err)
					logger.ErrorContext(ctx, "gRPC request failed",
						err,
						observability.NewField("method", info.FullMethod),
						observability.NewField("status_code", s.Code().String()),
						observability.NewField("duration_ms", duration.Milliseconds()),
					)

					// Set span status
					span.SetStatus(codes.Error, s.Message())
					span.SetAttributes(attribute.String("error.message", s.Message()))
				} else {
					logger.InfoContext(ctx, "gRPC request completed",
						observability.NewField("method", info.FullMethod),
						observability.NewField("duration_ms", duration.Milliseconds()),
					)

					// Set span status
					span.SetStatus(codes.Ok, "")
				}

				return resp, err
			})
		})
	}
}

// CombinedObservabilityStreamServerInterceptor creates a gRPC server interceptor that combines tracing, metrics, logging, and recovery
// for streaming RPCs.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.CombinedObservabilityStreamServerInterceptor(tracer, metrics, logger)),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//   - metrics: The Metrics instance to use for recording metrics
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func CombinedObservabilityStreamServerInterceptor(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) grpc.StreamServerInterceptor {
	// Create the individual interceptors
	tracingInterceptor := StreamServerTracingInterceptor(tracer)
	metricsInterceptor := MetricsStreamServerInterceptor(metrics)
	recoveryInterceptor := RecoveryStreamServerInterceptor(logger)

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Apply the recovery interceptor first to catch panics in other interceptors
		return recoveryInterceptor(srv, ss, info, func(srv interface{}, ss grpc.ServerStream) error {
			// Apply the tracing interceptor
			return tracingInterceptor(srv, ss, info, func(srv interface{}, ss grpc.ServerStream) error {
				// Get the current span from context
				ctx := ss.Context()
				span := trace.SpanFromContext(ctx)

				// Log the stream start with trace correlation
				logger.InfoContext(ctx, "gRPC stream started",
					observability.NewField("method", info.FullMethod),
					observability.NewField("trace_id", span.SpanContext().TraceID().String()),
					observability.NewField("span_id", span.SpanContext().SpanID().String()),
					observability.NewField("is_client_stream", info.IsClientStream),
					observability.NewField("is_server_stream", info.IsServerStream),
				)

				start := time.Now()

				// Apply the metrics interceptor
				err := metricsInterceptor(srv, ss, info, handler)

				// Calculate duration
				duration := time.Since(start)

				// Log the stream completion
				if err != nil {
					s, _ := status.FromError(err)
					logger.ErrorContext(ctx, "gRPC stream failed",
						err,
						observability.NewField("method", info.FullMethod),
						observability.NewField("status_code", s.Code().String()),
						observability.NewField("duration_ms", duration.Milliseconds()),
					)

					// Set span status
					span.SetStatus(codes.Error, s.Message())
					span.SetAttributes(attribute.String("error.message", s.Message()))
				} else {
					logger.InfoContext(ctx, "gRPC stream completed",
						observability.NewField("method", info.FullMethod),
						observability.NewField("duration_ms", duration.Milliseconds()),
					)

					// Set span status
					span.SetStatus(codes.Ok, "")
				}

				return err
			})
		})
	}
}
