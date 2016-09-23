package oauth2util

import (
	"strings"

	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"sourcegraph.com/sourcegraph/sourcegraph/pkg/auth"
)

// GRPCMiddleware reads the OAuth2 access token from the gRPC call's
// metadata. If present and valid, its information is added to the
// context.
//
// Lack of authentication is not an error, but a failed authentication
// attempt does result in a non-nil error.
func GRPCMiddleware(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromContext(ctx)
	if !ok {
		return ctx, nil
	}

	authMD, ok := md["authorization"]
	if !ok || len(authMD) == 0 {
		return ctx, nil
	}

	// This is for backwards compatibility with client instances that are running older versions
	// of sourcegraph (< v0.7.22).
	// TODO: remove this hack once clients upgrade to binaries having the new grpc-go API.
	authToken := authMD[len(authMD)-1]

	parts := strings.SplitN(authToken, " ", 2)
	if len(parts) != 2 {
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid authorization metadata")
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return ctx, nil
	}

	tokStr := parts[1]
	if tokStr == "" {
		return ctx, nil
	}

	return auth.AuthenticateByAccessToken(ctx, tokStr), nil
}
