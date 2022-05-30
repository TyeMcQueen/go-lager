package grpc_lager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/TyeMcQueen/go-lager"
	grpc_lager_testing "github.com/TyeMcQueen/go-lager/grpc/testing"
	pb_testproto "github.com/TyeMcQueen/go-lager/grpc/testproto"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
)

var (
	goodPing = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
)

type loggingPingService struct {
	pb_testproto.TestServiceServer
}

func (s *loggingPingService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	grpc_ctxtags.Extract(ctx).Set("custom_tags.string", "something").Set("custom_tags.int", 1337)
	lager.AddPairs(ctx, "custom_field", "custom_value")
	// lager.
	// ctxzap.AddFields(ctx, zap.String("custom_field", "custom_value"))
	// ctxzap.Extract(ctx).Info("some ping")
	return s.TestServiceServer.Ping(ctx, ping)
}

func (s *loggingPingService) PingError(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.Empty, error) {
	return s.TestServiceServer.PingError(ctx, ping)
}

// func (s *loggingPingService) PingList(ping *pb_testproto.PingRequest, stream pb_testproto.TestService_PingListServer) error {
// 	// grpc_ctxtags.Extract(stream.Context()).Set("custom_tags.string", "something").Set("custom_tags.int", 1337)
// 	// ctxzap.Extract(stream.Context()).Info("some pinglist")
// 	return s.TestServiceServer.PingList(ping, stream)
// }

// func (s *loggingPingService) PingEmpty(ctx context.Context, empty *pb_testproto.Empty) (*pb_testproto.PingResponse, error) {
// 	return s.TestServiceServer.PingEmpty(ctx, empty)
// }

type baseSuite struct {
	*grpc_lager_testing.InterceptorTestSuite
	mutexBuffer *grpc_testing.MutexReadWriter
	buffer      *bytes.Buffer
}

func newBaseSuite(t *testing.T) *baseSuite {
	// os.Setenv("LAGER_LEVELS", "FWNAI")

	b := &bytes.Buffer{}
	muB := grpc_testing.NewMutexReadWriter(b)
	lager.Init("FWNAI")
	lager.OutputDest = muB

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
