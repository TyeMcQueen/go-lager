/*
grpc_lager_test is a testing suite for testing grpc_lager
Based on test suite provided by https://github.com/grpc-ecosystem/go-grpc-middleware/tree/v1.3.0
*/
package grpc_lager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/TyeMcQueen/go-lager"
	grpc_lager "github.com/TyeMcQueen/go-lager/grpc_lager"
	"google.golang.org/grpc/codes"

	grpc_lager_testing "github.com/TyeMcQueen/go-lager/grpc_lager/testing"
	pb_testproto "github.com/TyeMcQueen/go-lager/grpc_lager/testproto"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
)

var (
	goodPing = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
)

func getMap(m interface{}) map[string]interface{} {
	newMap := m.(map[string]interface{})
	return newMap
}

type loggingPingService struct {
	pb_testproto.TestServiceServer
}

func (s *loggingPingService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	grpc_ctxtags.Extract(ctx).Set("custom_tags.string", "something").Set("custom_tags.int", 1337)
	ctx = lager.AddPairs(ctx, "custom_field", "custom_value")
	grpc_lager.Extract(ctx, 'I').MMap("some ping")

	return s.TestServiceServer.Ping(ctx, ping)
}

func (s *loggingPingService) PingError(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.Empty, error) {
	return s.TestServiceServer.PingError(ctx, ping)
}

type baseSuite struct {
	*grpc_lager_testing.InterceptorTestSuite
	mutexBuffer     *grpc_testing.MutexReadWriter
	buffer          *bytes.Buffer
	timestampFormat string
}

func newBaseSuite(t *testing.T, levels string) *baseSuite {
	b := &bytes.Buffer{}
	muB := grpc_testing.NewMutexReadWriter(b)
	lager.Init(levels)
	lager.SetOutput(muB)

	return &baseSuite{
		buffer:      b,
		mutexBuffer: muB,
		InterceptorTestSuite: &grpc_lager_testing.InterceptorTestSuite{
			TestService: &loggingPingService{&grpc_lager_testing.TestPingService{T: t}},
		},
	}
}

func (s *baseSuite) SetupTest() {
	s.mutexBuffer.Lock()
	s.buffer.Reset()
	s.mutexBuffer.Unlock()
}

func (s *baseSuite) getOutputJSONs() [][]interface{} {
	ret := make([][]interface{}, 0)
	dec := json.NewDecoder(s.mutexBuffer)

	for {
		var val []interface{}
		err := dec.Decode(&val)
		if err == io.EOF {
			break
		}
		if err != nil {
			s.T().Fatalf("failed decoding output from Lager JSON: %v", err)
		}

		ret = append(ret, val)
	}

	return ret
}

func StubMessageProducer(ctx context.Context, msg string, level byte, code codes.Code, err error, duration *lager.KVPairs) {
	// re-extract logger from newCtx, as it may have extra fields that changed in the holder.
	ctx = lager.ContextPairs(ctx).Merge(duration).InContext(ctx)
	lager.Level(level, ctx).MMap("custom message",
		"grpc.code", code,
		lager.Unless(nil == err, "error"), err,
	)
}
