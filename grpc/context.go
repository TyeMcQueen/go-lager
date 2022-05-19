package grpc_lager

import (
	"context"

	"github.com/TyeMcQueen/go-lager"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
)

func TagsToPairs(ctx context.Context) lager.AMap {
	tags := grpc_ctxtags.Extract(ctx)
	pairs := lager.AMap(nil)

	for k, v := range tags.Values() {
		pairs.AddPairs(k, v)
	}

	return pairs
}
