package security

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthorizationUnaryServerInterceptor creates a gRPC server interceptor for authorization in unary RPCs
func AuthorizationUnaryServerInterceptor(authorizer Authorizer, resourceType string, action Action) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Get claims from context
		claims, ok := GetClaimsFromContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "No authentication claims found")
		}

		// Create subject
		subject := Subject{
			ID:         claims.Subject,
			Attributes: claims.Custom,
		}

		// Get roles and permissions from context
		if roles, ok := authorizer.GetRoles(ctx); ok {
			subject.Roles = roles
		}
		if permissions, ok := authorizer.GetPermissions(ctx); ok {
			subject.Permissions = permissions
		}

		// Create resource
		resource := Resource{
			Type: resourceType,
			ID:   info.FullMethod,
			Attributes: map[string]interface{}{
				"method": info.FullMethod,
			},
		}

		// Create authorization request
		authReq := AuthorizationRequest{
			Subject:  subject,
			Resource: resource,
			Action:   action,
			Context: map[string]interface{}{
				"method": info.FullMethod,
			},
		}

		// Authorize the request
		resp, err := authorizer.Authorize(ctx, authReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Authorization error: %v", err)
		}

		if !resp.Allowed {
			return nil, status.Errorf(codes.PermissionDenied, "Forbidden: %s", resp.Reason)
		}

		// Call the next handler
		return handler(ctx, req)
	}
}

// AuthorizationStreamServerInterceptor creates a gRPC server interceptor for authorization in streaming RPCs
func AuthorizationStreamServerInterceptor(authorizer Authorizer, resourceType string, action Action) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Get claims from context
		claims, ok := GetClaimsFromContext(ctx)
		if !ok {
			return status.Errorf(codes.Unauthenticated, "No authentication claims found")
		}

		// Create subject
		subject := Subject{
			ID:         claims.Subject,
			Attributes: claims.Custom,
		}

		// Get roles and permissions from context
		if roles, ok := authorizer.GetRoles(ctx); ok {
			subject.Roles = roles
		}
		if permissions, ok := authorizer.GetPermissions(ctx); ok {
			subject.Permissions = permissions
		}

		// Create resource
		resource := Resource{
			Type: resourceType,
			ID:   info.FullMethod,
			Attributes: map[string]interface{}{
				"method": info.FullMethod,
			},
		}

		// Create authorization request
		authReq := AuthorizationRequest{
			Subject:  subject,
			Resource: resource,
			Action:   action,
			Context: map[string]interface{}{
				"method": info.FullMethod,
			},
		}

		// Authorize the request
		resp, err := authorizer.Authorize(ctx, authReq)
		if err != nil {
			return status.Errorf(codes.Internal, "Authorization error: %v", err)
		}

		if !resp.Allowed {
			return status.Errorf(codes.PermissionDenied, "Forbidden: %s", resp.Reason)
		}

		// Call the next handler
		return handler(srv, ss)
	}
}

// RequireRoleUnaryInterceptor creates a gRPC server interceptor that requires a specific role
func RequireRoleUnaryInterceptor(authorizer Authorizer, role Role) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if the user has the required role
		hasRole, err := authorizer.HasRole(ctx, role)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Authentication error: %v", err)
		}

		if !hasRole {
			return nil, status.Errorf(codes.PermissionDenied, "Missing required role: %s", role)
		}

		// Call the next handler
		return handler(ctx, req)
	}
}

// RequireRoleStreamInterceptor creates a gRPC server interceptor that requires a specific role
func RequireRoleStreamInterceptor(authorizer Authorizer, role Role) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Check if the user has the required role
		hasRole, err := authorizer.HasRole(ctx, role)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "Authentication error: %v", err)
		}

		if !hasRole {
			return status.Errorf(codes.PermissionDenied, "Missing required role: %s", role)
		}

		// Call the next handler
		return handler(srv, ss)
	}
}

// RequirePermissionUnaryInterceptor creates a gRPC server interceptor that requires a specific permission
func RequirePermissionUnaryInterceptor(authorizer Authorizer, permission Permission) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if the user has the required permission
		hasPermission, err := authorizer.HasPermission(ctx, permission)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "Authentication error: %v", err)
		}

		if !hasPermission {
			return nil, status.Errorf(codes.PermissionDenied, "Missing required permission: %s", permission)
		}

		// Call the next handler
		return handler(ctx, req)
	}
}

// RequirePermissionStreamInterceptor creates a gRPC server interceptor that requires a specific permission
func RequirePermissionStreamInterceptor(authorizer Authorizer, permission Permission) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Check if the user has the required permission
		hasPermission, err := authorizer.HasPermission(ctx, permission)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "Authentication error: %v", err)
		}

		if !hasPermission {
			return status.Errorf(codes.PermissionDenied, "Missing required permission: %s", permission)
		}

		// Call the next handler
		return handler(srv, ss)
	}
}

// RolesAndPermissionsUnaryInterceptor creates a gRPC server interceptor that extracts roles and permissions from JWT claims
func RolesAndPermissionsUnaryInterceptor(roleClaim, permissionClaim string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract roles and permissions
		roles, permissions := ExtractRolesAndPermissions(ctx, roleClaim, permissionClaim)

		// Add to context
		if len(roles) > 0 {
			ctx = context.WithValue(ctx, RolesContextKey, roles)
		}
		if len(permissions) > 0 {
			ctx = context.WithValue(ctx, PermissionsContextKey, permissions)
		}

		// Call the next handler with the updated context
		return handler(ctx, req)
	}
}

// RolesAndPermissionsStreamInterceptor creates a gRPC server interceptor that extracts roles and permissions from JWT claims
func RolesAndPermissionsStreamInterceptor(roleClaim, permissionClaim string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Extract roles and permissions
		roles, permissions := ExtractRolesAndPermissions(ctx, roleClaim, permissionClaim)

		// Add to context
		if len(roles) > 0 {
			ctx = context.WithValue(ctx, RolesContextKey, roles)
		}
		if len(permissions) > 0 {
			ctx = context.WithValue(ctx, PermissionsContextKey, permissions)
		}

		// Create a wrapped server stream with the updated context
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Call the next handler with the updated context
		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream wraps a grpc.ServerStream with a new context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
