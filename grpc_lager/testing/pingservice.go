/*
Package `grpc_lager_testing` provides helper functions for testing validators in this package.
*/

package grpc_lager_testing

import (
	"context"
	"testing"

	pb_testproto "github.com/TyeMcQueen/go-lager/grpc/testproto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// DefaultPongValue is the default value used.
	DefaultResponseValue = "default_response_value"
	// ListResponseCount is the expected number of responses to PingList
	ListResponseCount = 100
)

type TestPingService struct {
	pb_testproto.TestServiceServer
	T *testing.T
}

func (s *TestPingService) PingEmpty(ctx context.Context, _ *pb_testproto.Empty) (*pb_testproto.PingResponse, error) {
	return &pb_testproto.PingResponse{Value: DefaultResponseValue, Counter: 42}, nil
}

func (s *TestPingService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	// Send user trailers and headers.
	return &pb_testproto.PingResponse{Value: ping.Value, Counter: 42}, nil
}

func (s *TestPingService) PingError(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.Empty, error) {
	code := codes.Code(ping.ErrorCodeReturned)
	return nil, status.Errorf(code, "Userspace error.")
}
