package grpc_lager

import (
	"context"
	"path"

	"github.com/TyeMcQueen/go-lager"
	"google.golang.org/grpc"
)

// based on https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/logging/zap/server_interceptors.go

var (
	// SystemField is used in every log statement made through grpc_lager. Can be overwritten before any initialization code.
	SystemField = lager.RawMap{"system", "grpc"}

	// ServerField is used in every server-side log statement made through grpc_lager.Can be overwritten before initialization.
	ServerField = lager.RawMap{"span.kind", "server"}
)

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

func serverCallFields(fullMethodString string) lager.RawMap {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)

	return append(SystemField, append(ServerField, lager.RawMap{
		"grpc.service", service,
		"grpc.method", method,
	}...)...)
}
