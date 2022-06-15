package grpc_lager_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	grpc_lager "github.com/TyeMcQueen/go-lager/grpc_lager"
	pb_testproto "github.com/TyeMcQueen/go-lager/grpc_lager/testproto"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func customCodeToLevel(c codes.Code) byte {
	if c == codes.Unauthenticated {
		// Make this a special case for tests, and an error.
		return 'A'
	}
	level := grpc_lager.DefaultCodeToLevel(c)
	return level
}

func TestLagerGrpcLoggingSuite(t *testing.T) {
	if strings.HasPrefix(runtime.Version(), "go1.7") {
		t.Skipf("Skipping due to json.RawMessage incompatibility with go1.7")
		return
	}

	for _, tcase := range []struct {
		timestampFormat string
	}{
		{
			timestampFormat: time.RFC3339,
		},
		{
			timestampFormat: "2006-01-02",
		},
	} {
		opts := []grpc_lager.Option{
			grpc_lager.WithLevels(customCodeToLevel),
			grpc_lager.WithTimestampFormat(tcase.timestampFormat),
		}
		b := newBaseSuite(t, "FWNAEIWP")
		b.timestampFormat = tcase.timestampFormat
		b.InterceptorTestSuite.ServerOpts = []grpc.ServerOption{
			grpc_middleware.WithUnaryServerChain(
				grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
				grpc_lager.UnaryServerInterceptor(opts...)),
		}
		suite.Run(t, &serverSuite{b})
	}
}

type serverSuite struct {
	*baseSuite
}

func (s *serverSuite) TestPing_WithCustomTags() {
	deadline := time.Now().Add(5 * time.Second)
	_, err := s.Client.Ping(s.DeadlineCtx(deadline), goodPing)
	require.NoError(s.T(), err, "there must be not be an error on a successful call")

	msgs := s.getOutputJSONs()
	require.Len(s.T(), msgs, 2, "two log statements should be logged")
	for _, m := range msgs {
		last := getMap(m[len(m)-1])
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "Ping", last["grpc.method"], "all lines must contain method name")
		assert.Equal(s.T(), "server", last["span.kind"], "all lines must contain the kind of call (server)")
		assert.Equal(s.T(), "something", last["custom_tags.string"], "all lines must contain `custom_tags.string`")

		assert.Contains(s.T(), last, "custom_tags.int", "all lines must contain `custom_tags.int`")
		require.Contains(s.T(), last, "grpc.start_time", "all lines must contain the start time")
		_, err := time.Parse(s.timestampFormat, last["grpc.start_time"].(string))
		assert.NoError(s.T(), err, "should be able to parse start time")

		require.Contains(s.T(), last, "grpc.request.deadline", "all lines must contain the deadline of the call")
		_, err = time.Parse(s.timestampFormat, last["grpc.request.deadline"].(string))
		require.NoError(s.T(), err, "should be able to parse deadline")
		assert.Equal(s.T(), last["grpc.request.deadline"], deadline.Format(s.timestampFormat), "should have the same deadline that was set by the caller")
	}

	// The message logged in the gRPC service handler directly after adding pairs to the context should contain the custom_field,
	// any message logged in an interceptor will not have this new context since it is never passed. This is expected behavior,
	// other logging libraries work around this by updating values of an existing context rather than creating a new one.
	assert.Equal(s.T(), "custom_value", getMap(msgs[0][4])["custom_field"], "first message must contain `custom_field`")

	assert.Equal(s.T(), "some ping", msgs[0][2], "handler's message must contain user message")

	assert.Equal(s.T(), "finished unary call with code OK", msgs[1][2], "handler's message must contain user message")
	assert.Equal(s.T(), "INFO", msgs[1][1], "must be logged at info level")
	assert.Contains(s.T(), msgs[1][4], "grpc.time_ms", "interceptor log statement should contain execution time")
}

func (s *serverSuite) TestPingError_WithCustomLevels() {
	for _, tcase := range []struct {
		code  codes.Code
		level string
		msg   string
	}{
		{
			code:  codes.Internal,
			level: "FAIL",
			msg:   "Internal must remap to Fail level in DefaultCodeToLevel",
		},
		{
			code:  codes.NotFound,
			level: "INFO",
			msg:   "NotFound must remap to Info level in DefaultCodeToLevel",
		},
		{
			code:  codes.FailedPrecondition,
			level: "WARN",
			msg:   "FailedPrecondition must remap to Warn level in DefaultCodeToLevel",
		},
		{
			code:  codes.Unauthenticated,
			level: "ACCESS",
			msg:   "Unauthenticated is overwritten to Panic level with customCodeToLevel override, which probably didn't work",
		},
	} {
		s.buffer.Reset()
		_, err := s.Client.PingError(
			s.SimpleCtx(),
			&pb_testproto.PingRequest{Value: "something", ErrorCodeReturned: uint32(tcase.code)})
		require.Error(s.T(), err, "each call here must return an error")

		msgs := s.getOutputJSONs()
		require.Len(s.T(), msgs, 1, "only the interceptor log message is printed in PingErr")

		m := msgs[0]
		last := getMap(m[len(m)-1])
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "PingError", last["grpc.method"], "all lines must contain method name")
		assert.Equal(s.T(), tcase.code.String(), getMap(m[3])["grpc.code"], "all lines have the correct gRPC code")
		assert.Equal(s.T(), tcase.level, m[1], tcase.msg)
		assert.Equal(s.T(), "finished unary call with code "+tcase.code.String(), m[2], "needs the correct end message")

		require.Contains(s.T(), last, "grpc.start_time", "all lines must contain the start time")
		_, err = time.Parse(s.timestampFormat, last["grpc.start_time"].(string))
		assert.NoError(s.T(), err, "should be able to parse start time")
	}
}

func TestLagerGrpcLoggingOverrideSuite(t *testing.T) {
	if strings.HasPrefix(runtime.Version(), "go1.7") {
		t.Skip("Skipping due to json.RawMessage incompatibility with go1.7")
		return
	}

	opts := []grpc_lager.Option{
		grpc_lager.WithDurationField(grpc_lager.DurationToDurationField),
	}
	b := newBaseSuite(t, "FWNAEIWP")
	b.InterceptorTestSuite.ServerOpts = []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_lager.UnaryServerInterceptor(opts...)),
	}
	suite.Run(t, &serverOverrideSuite{b})
}

type serverOverrideSuite struct {
	*baseSuite
}

func (s *serverOverrideSuite) TestPing_HasOverriddenDuration() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "there must be not be an error on a successful call")
	msgs := s.getOutputJSONs()
	require.Len(s.T(), msgs, 2, "two log statements should be logged")

	for _, m := range msgs {
		last := getMap(m[len(m)-1])
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "Ping", last["grpc.method"], "all lines must contain method name")
	}

	assert.Equal(s.T(), "some ping", msgs[0][2], "handler's message must contain user message")
	assert.NotContains(s.T(), msgs[0], "grpc.time_ms", "handler's message must not contain default duration")
	assert.NotContains(s.T(), msgs[0], "grpc.duration", "handler's message must not contain overridden duration")

	assert.Equal(s.T(), "finished unary call with code OK", msgs[1][2], "handler's message must contain user message")
	assert.Equal(s.T(), "INFO", msgs[1][1], "OK error codes must be logged on info level.")
	assert.NotContains(s.T(), getMap(msgs[1][4]), "grpc.time_ms", "handler's message must not contain default duration")
	assert.Contains(s.T(), getMap(msgs[1][4]), "grpc.duration", "handler's message must contain overridden duration")
}

func TestLagerGrpcServerOverrideSuppressedSuite(t *testing.T) {
	if strings.HasPrefix(runtime.Version(), "go1.7") {
		t.Skip("Skipping due to json.RawMessage incompatibility with go1.7")
		return
	}
	opts := []grpc_lager.Option{
		grpc_lager.WithDecider(func(method string, err error) bool {
			if err != nil && method == "/lager_grpc.testproto.TestService/PingError" {
				return true
			}
			return false
		}),
	}
	b := newBaseSuite(t, "FWNAEIWP")
	b.InterceptorTestSuite.ServerOpts = []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_lager.UnaryServerInterceptor(opts...)),
	}
	suite.Run(t, &serverOverriddenDeciderSuite{b})
}

type serverOverriddenDeciderSuite struct {
	*baseSuite
}

func (s *serverOverriddenDeciderSuite) TestPing_HasOverriddenDecider() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "there must be not be an error on a successful call")
	msgs := s.getOutputJSONs()
	require.Len(s.T(), msgs, 1, "single log statements should be logged")

	last := getMap(msgs[0][4])
	assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
	assert.Equal(s.T(), "Ping", last["grpc.method"], "all lines must contain method name")
	assert.Equal(s.T(), "some ping", msgs[0][2], "handler's message must contain user message")
}

func (s *serverOverriddenDeciderSuite) TestPingError_HasOverriddenDecider() {
	code := codes.NotFound
	msg := "NotFound must remap to InfoLevel in DefaultCodeToLevel"

	s.buffer.Reset()
	_, err := s.Client.PingError(
		s.SimpleCtx(),
		&pb_testproto.PingRequest{Value: "something", ErrorCodeReturned: uint32(code)})
	require.Error(s.T(), err, "each call here must return an error")
	msgs := s.getOutputJSONs()
	require.Len(s.T(), msgs, 1, "only the interceptor log message is printed in PingErr")
	m := msgs[0]
	last := getMap(m[len(m)-1])
	assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
	assert.Equal(s.T(), "PingError", last["grpc.method"], "all lines must contain method name")
	assert.Equal(s.T(), code.String(), getMap(m[3])["grpc.code"], "all lines must contain the correct gRPC code")
	assert.Equal(s.T(), "INFO", m[1], msg)
}

func TestLagerGrpcLoggingServerMessageProducerSuite(t *testing.T) {
	if strings.HasPrefix(runtime.Version(), "go1.7") {
		t.Skip("Skipping due to json.RawMessage incompatibility with go1.7")
		return
	}
	opts := []grpc_lager.Option{
		grpc_lager.WithMessageProducer(StubMessageProducer),
	}
	b := newBaseSuite(t, "FWNAEIWP")
	b.InterceptorTestSuite.ServerOpts = []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_lager.UnaryServerInterceptor(opts...)),
	}
	suite.Run(t, &serverMessageProducerSuite{b})
}

type serverMessageProducerSuite struct {
	*baseSuite
}

func (s *serverMessageProducerSuite) TestPing_HasOverriddenMessageProducer() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "there must be not be an error on a successful call")
	msgs := s.getOutputJSONs()
	require.Len(s.T(), msgs, 2, "two log statements should be logged")

	for _, m := range msgs {
		last := getMap(m[len(m)-1])
		assert.Equal(s.T(), "lager_grpc.testproto.TestService", last["grpc.service"], "all lines must contain service name")
		assert.Equal(s.T(), "Ping", last["grpc.method"], "all lines must contain method name")
	}
	assert.Equal(s.T(), "some ping", msgs[0][2], "handler's message must contain user message")

	assert.Equal(s.T(), "custom message", msgs[1][2], "handler's message must contain user message")
	assert.Equal(s.T(), "INFO", msgs[1][1], "OK error codes must be logged on info level.")
}
