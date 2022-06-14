package grpc_lager

import (
	"context"

	"github.com/TyeMcQueen/go-lager"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
)

// TagsToPairs extracts the tags provided by the go-grpc-middleware library from
// the context, adds them to the context as Lager pairs and returns an updated context
func TagsToPairs(ctx context.Context) context.Context {
	tags := grpc_ctxtags.Extract(ctx)

	for k, v := range tags.Values() {
		ctx = lager.AddPairs(ctx, k, v)
	}

	return ctx
}

// Pass in context and one character from "PEFWNAITDOG" to
// get a Lager object that has all the grpc_ctxtags updated.
func Extract(ctx context.Context, lev byte) lager.Lager {
	ctx = TagsToPairs(ctx)

	return lager.Level(lev, ctx)
}
