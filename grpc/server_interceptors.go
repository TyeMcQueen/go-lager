package grpc_lager

import (
	"context"
	"path"
	"time"

	"github.com/TyeMcQueen/go-lager"
	"google.golang.org/grpc"
)

// based on https://github.com/grpc-ecosystem/go-grpc-middleware/blob/master/logging/zap/server_interceptors.go

var (
	// SystemField is used in every log statement made through grpc_lager. Can be overwritten before any initialization code.
	SystemField = lager.Pairs("system", "grpc")

	// ServerField is used in every server-side log statement made through grpc_lager. Can be overwritten before initialization.
	ServerField = lager.Pairs("span.kind", "server")
)

func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(opts)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		newCtx := newContextForCall(ctx, info.FullMethod, startTime, o.timestampFormat)

		resp, err := handler(newCtx, req)
		if !o.shouldLog(info.FullMethod, err) {
			return resp, err
		}
		code := o.codeFunc(err)
		level := o.levelFunc(code)
		duration := o.durationFunc(time.Since(startTime))

		o.messageFunc(newCtx, "finished unary call with code "+code.String(), level, code, err, duration)

		return resp, err
	}
}

func newContextForCall(ctx context.Context, fullMethodString string, start time.Time, timestampFormat string) context.Context {
	pairs := lager.Pairs("grpc.start_time", start.Format(timestampFormat))
	if d, ok := ctx.Deadline(); ok {
		pairs.AddPairs("grpc.request.deadline", d.Format(timestampFormat))
	}

	return lager.ContextPairs(ctx).Merge(pairs).Merge(serverCallFields(fullMethodString)).InContext(ctx)
}

func serverCallFields(fullMethodString string) *lager.KVPairs {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)

	return lager.Pairs(
		"grpc.service", service,
		"grpc.method", method,
	).Merge(SystemField).Merge(ServerField)
}
