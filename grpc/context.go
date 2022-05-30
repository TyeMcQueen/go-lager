package grpc_lager

import (
	"context"

	"github.com/TyeMcQueen/go-lager"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
)

// TagsToPairs extracts the tags provided by the go-grpc-middleware library from
// the context and returns a lager map
func TagsToPairs(ctx context.Context) lager.AMap {
	tags := grpc_ctxtags.Extract(ctx)
	pairs := lager.AMap(nil)

	for k, v := range tags.Values() {
		pairs.AddPairs(k, v)
	}

	return pairs
}
