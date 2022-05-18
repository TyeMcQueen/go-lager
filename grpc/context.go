package grpc_lager

import (
	"context"

	"github.com/TyeMcQueen/go-lager"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
)

func TagsToPairs(ctx context.Context) lager.RawMap {
	tags := grpc_ctxtags.Extract(ctx)
	var pairs lager.RawMap

	for k, v := range tags.Values() {
		pairs = append(pairs, k, v)
	}

	return pairs
}
