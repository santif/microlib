package grpc

import (
	"context"

	"github.com/santif/microlib/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerTracingInterceptor creates a gRPC server interceptor for tracing unary RPCs.
// This interceptor automatically creates spans for each gRPC request and propagates
// trace context through gRPC metadata.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.UnaryServerTracingInterceptor(tracer),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func UnaryServerTracingInterceptor(tracer observability.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract trace context from incoming metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		// Create a carrier from the metadata
		carrier := metadataCarrier(md)

		// Extract the context
		ctx = tracer.Extract(ctx, carrier)

		// Start a new span for this RPC
		spanName := info.FullMethod
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(info.FullMethod)),
				attribute.String("rpc.method", extractMethod(info.FullMethod)),
			),
		)
		defer span.End()

		// Process the request
		resp, err := handler(ctx, req)

		// Set status code based on the error
		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int64(int64(s.Code())),
				attribute.String("error.message", s.Message()),
			)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return resp, err
	}
}

// StreamServerTracingInterceptor creates a gRPC server interceptor for tracing streaming RPCs.
// This interceptor automatically creates spans for each gRPC streaming request and propagates
// trace context through gRPC metadata.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.StreamServerTracingInterceptor(tracer),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func StreamServerTracingInterceptor(tracer observability.Tracer) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Extract trace context from incoming metadata
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		// Create a carrier from the metadata
		carrier := metadataCarrier(md)

		// Extract the context
		ctx = tracer.Extract(ctx, carrier)

		// Start a new span for this RPC
		spanName := info.FullMethod
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(info.FullMethod)),
				attribute.String("rpc.method", extractMethod(info.FullMethod)),
				attribute.Bool("rpc.grpc.streaming", true),
			),
		)
		defer span.End()

		// Wrap the server stream to use the new context with the span
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Process the request
		err := handler(srv, wrappedStream)

		// Set status code based on the error
		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int64(int64(s.Code())),
				attribute.String("error.message", s.Message()),
			)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// UnaryClientTracingInterceptor creates a gRPC client interceptor for tracing unary RPCs.
// This interceptor automatically creates spans for each gRPC client request and propagates
// trace context through gRPC metadata.
//
// Usage:
//
//	conn, err := grpc.Dial(
//	    address,
//	    grpc.WithUnaryInterceptor(grpc.ChainUnaryInterceptor(
//	        grpc.UnaryClientTracingInterceptor(tracer),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//
// Returns:
//   - A gRPC UnaryClientInterceptor function
func UnaryClientTracingInterceptor(tracer observability.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Start a new span for this RPC
		spanName := method
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(method)),
				attribute.String("rpc.method", extractMethod(method)),
				attribute.String("net.peer.name", cc.Target()),
			),
		)
		defer span.End()

		// Extract metadata from the context
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			// Clone the metadata to avoid modifying the context's metadata
			md = md.Copy()
		}

		// Create a carrier from the metadata
		carrier := metadataCarrier(md)

		// Inject the trace context into the carrier
		tracer.Inject(ctx, carrier)

		// Create a new context with the updated metadata
		ctx = metadata.NewOutgoingContext(ctx, metadata.MD(carrier))

		// Invoke the RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Set status code based on the error
		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int64(int64(s.Code())),
				attribute.String("error.message", s.Message()),
			)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// StreamClientTracingInterceptor creates a gRPC client interceptor for tracing streaming RPCs.
// This interceptor automatically creates spans for each gRPC client streaming request and propagates
// trace context through gRPC metadata.
//
// Usage:
//
//	conn, err := grpc.Dial(
//	    address,
//	    grpc.WithStreamInterceptor(grpc.ChainStreamInterceptor(
//	        grpc.StreamClientTracingInterceptor(tracer),
//	        // other interceptors...
//	    )),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//
// Returns:
//   - A gRPC StreamClientInterceptor function
func StreamClientTracingInterceptor(tracer observability.Tracer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Start a new span for this RPC
		spanName := method
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.service", extractService(method)),
				attribute.String("rpc.method", extractMethod(method)),
				attribute.String("net.peer.name", cc.Target()),
				attribute.Bool("rpc.grpc.streaming", true),
			),
		)

		// Extract metadata from the context
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			// Clone the metadata to avoid modifying the context's metadata
			md = md.Copy()
		}

		// Create a carrier from the metadata
		carrier := metadataCarrier(md)

		// Inject the trace context into the carrier
		tracer.Inject(ctx, carrier)

		// Create a new context with the updated metadata
		ctx = metadata.NewOutgoingContext(ctx, metadata.MD(carrier))

		// Invoke the streamer
		stream, err := streamer(ctx, desc, cc, method, opts...)

		// Set status code based on the error
		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(
				semconv.RPCGRPCStatusCodeKey.Int64(int64(s.Code())),
				attribute.String("error.message", s.Message()),
			)
			span.End()
			return nil, err
		}

		// Return a wrapped stream that will end the span when the stream is closed
		return &wrappedClientStream{
			ClientStream: stream,
			span:         span,
		}, nil
	}
}

// wrappedServerStream wraps a grpc.ServerStream to override the Context method
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// wrappedClientStream wraps a grpc.ClientStream to end the span when the stream is closed
type wrappedClientStream struct {
	grpc.ClientStream
	span trace.Span
}

// RecvMsg overrides the RecvMsg method to end the span when the stream is closed
func (w *wrappedClientStream) RecvMsg(m interface{}) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		// End the span when the stream is closed
		w.span.End()
	}
	return err
}

// SendMsg overrides the SendMsg method to end the span when the stream is closed
func (w *wrappedClientStream) SendMsg(m interface{}) error {
	err := w.ClientStream.SendMsg(m)
	if err != nil {
		// End the span when the stream is closed
		w.span.End()
	}
	return err
}

// CloseSend overrides the CloseSend method to end the span when the stream is closed
func (w *wrappedClientStream) CloseSend() error {
	err := w.ClientStream.CloseSend()
	// Note: We don't end the span here because the client might still receive messages
	return err
}

// metadataCarrier adapts gRPC metadata to the TextMapCarrier interface
type metadataCarrier metadata.MD

// Get returns the value associated with the passed key.
func (mc metadataCarrier) Get(key string) string {
	values := metadata.MD(mc).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set stores the key-value pair.
func (mc metadataCarrier) Set(key string, value string) {
	metadata.MD(mc).Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (mc metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range metadata.MD(mc) {
		keys = append(keys, k)
	}
	return keys
}

// extractService extracts the service name from the full method name
// Format: /service/method
func extractService(fullMethod string) string {
	if fullMethod[0] == '/' {
		fullMethod = fullMethod[1:]
	}
	pos := 0
	for i, c := range fullMethod {
		if c == '/' {
			pos = i
			break
		}
	}
	if pos == 0 {
		return ""
	}
	return fullMethod[:pos]
}

// extractMethod extracts the method name from the full method name
// Format: /service/method
func extractMethod(fullMethod string) string {
	if fullMethod[0] == '/' {
		fullMethod = fullMethod[1:]
	}
	pos := 0
	for i, c := range fullMethod {
		if c == '/' {
			pos = i
			break
		}
	}
	if pos == 0 || pos == len(fullMethod)-1 {
		return fullMethod
	}
	return fullMethod[pos+1:]
}

// CombinedUnaryServerInterceptor creates a gRPC server interceptor that combines tracing, metrics, and logging.
// This interceptor applies all three observability concerns in a single interceptor function.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpc.CombinedUnaryServerInterceptor(tracer, metrics, logger)),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//   - metrics: The Metrics instance to use for recording metrics
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC UnaryServerInterceptor function
func CombinedUnaryServerInterceptor(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) grpc.UnaryServerInterceptor {
	// Create the individual interceptors
	tracingInterceptor := UnaryServerTracingInterceptor(tracer)
	// Note: Metrics interceptor would be added here when implemented

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
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

			// Process the request
			resp, err := handler(ctx, req)

			// Log the response
			if err != nil {
				s, _ := status.FromError(err)
				logger.ErrorContext(ctx, "gRPC request failed",
					err,
					observability.NewField("method", info.FullMethod),
					observability.NewField("status_code", s.Code().String()),
				)
			} else {
				logger.InfoContext(ctx, "gRPC request completed",
					observability.NewField("method", info.FullMethod),
				)
			}

			return resp, err
		})
	}
}

// CombinedStreamServerInterceptor creates a gRPC server interceptor that combines tracing, metrics, and logging
// for streaming RPCs.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StreamInterceptor(grpc.CombinedStreamServerInterceptor(tracer, metrics, logger)),
//	)
//
// Parameters:
//   - tracer: The Tracer instance to use for creating spans
//   - metrics: The Metrics instance to use for recording metrics
//   - logger: The Logger instance to use for logging
//
// Returns:
//   - A gRPC StreamServerInterceptor function
func CombinedStreamServerInterceptor(tracer observability.Tracer, metrics observability.Metrics, logger observability.Logger) grpc.StreamServerInterceptor {
	// Create the individual interceptors
	tracingInterceptor := StreamServerTracingInterceptor(tracer)
	// Note: Metrics interceptor would be added here when implemented

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Apply the tracing interceptor
		return tracingInterceptor(srv, ss, info, func(srv interface{}, stream grpc.ServerStream) error {
			// Get the current span from context
			ctx := stream.Context()
			span := trace.SpanFromContext(ctx)

			// Log the request with trace correlation
			logger.InfoContext(ctx, "gRPC stream started",
				observability.NewField("method", info.FullMethod),
				observability.NewField("trace_id", span.SpanContext().TraceID().String()),
				observability.NewField("span_id", span.SpanContext().SpanID().String()),
			)

			// Process the stream
			err := handler(srv, stream)

			// Log the response
			if err != nil {
				s, _ := status.FromError(err)
				logger.ErrorContext(ctx, "gRPC stream failed",
					err,
					observability.NewField("method", info.FullMethod),
					observability.NewField("status_code", s.Code().String()),
				)
			} else {
				logger.InfoContext(ctx, "gRPC stream completed",
					observability.NewField("method", info.FullMethod),
				)
			}

			return err
		})
	}
}
