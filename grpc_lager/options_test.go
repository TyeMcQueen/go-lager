package grpc_lager_test

import (
	"context"
	"testing"
	"time"

	"github.com/TyeMcQueen/go-lager"
	lager_grpc "github.com/TyeMcQueen/go-lager/grpc"
	"github.com/TyeMcQueen/go-tutl"
)

func TestDurationToTimeMillisField(t *testing.T) {
	u := tutl.New(t)
	expectedCtx := lager.Pairs("grpc.time_ms", float32(0.1)).InContext(context.TODO())

	ctx := lager_grpc.DurationToTimeMillisField(time.Microsecond * 100).InContext(context.TODO())

	u.Is(expectedCtx, ctx, "sub millisecond values in context should be correct")
}
