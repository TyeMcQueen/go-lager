package grpc_lager_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	grpc_lager "github.com/TyeMcQueen/go-lager/grpc"
	pb_testproto "github.com/TyeMcQueen/go-lager/grpc/testproto"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
)

func TestLagerGrpcPayloadSuite(t *testing.T) {
	if strings.HasPrefix(runtime.Version(), "go1.7") {
		t.Skipf("Skipping due to json.RawMessage incompatibility with go1.7")
		return
	}
	alwaysLoggingDeciderServer := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool { return true }

	b := newBaseSuite(t, "FWNA")

	b.InterceptorTestSuite.ServerOpts = []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_lager.PayloadUnaryServerInterceptor(alwaysLoggingDeciderServer),
		),
	}

	suite.Run(t, &payloadSuite{b})
}

type payloadSuite struct {
	*baseSuite
}

func (s *payloadSuite) getServerMessages(expectedServer int) (serverMsgs [][]interface{}) {
	msgs := s.getOutputJSONs()
	for _, m := range msgs {
		last := m[len(m)-1].(map[string]interface{})
		if last["span.kind"] == "server" {
			serverMsgs = append(serverMsgs, m)
		}
	}
	require.Len(s.T(), msgs, expectedServer, "must match expected number of server log messages")
	return serverMsgs
}

func (s *payloadSuite) TestPing_LogsBothRequestAndResponse() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)

	require.NoError(s.T(), err, "there must be not be an error on a successful call")
	serverMsgs := s.getServerMessages(2)

	for _, m := range serverMsgs {
		level := m[1]
		last := m[len(m)-1].(map[string]interface{})
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "Ping", last["grpc.method"], "all lines must contain method name")
		assert.Equal(s.T(), "ACCESS", level, "all payloads must be logged on access level")
	}

	serverReq, serverResp := serverMsgs[0], serverMsgs[1]

	assert.Contains(s.T(), serverReq[2], "grpc.request.content", "request payload must be logged in a structured way")
	assert.Contains(s.T(), serverResp[2], "grpc.response.content", "response payload must be logged in a structured way")

}

func (s *payloadSuite) TestPingError_LogsOnlyRequestsOnError() {
	_, err := s.Client.PingError(s.SimpleCtx(), &pb_testproto.PingRequest{Value: "something", ErrorCodeReturned: uint32(4)})

	require.Error(s.T(), err, "there must be an error on an unsuccessful call")
	serverMsgs := s.getServerMessages(1)
	for _, m := range serverMsgs {
		level := m[1]
		last := m[len(m)-1].(map[string]interface{})
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "PingError", last["grpc.method"], "all lines must contain method name")
		assert.Equal(s.T(), "ACCESS", level, "must be logged at the access level")
	}

	assert.Contains(s.T(), serverMsgs[0][2], "grpc.request.content", "request payload must be logged in a structured way")
}
