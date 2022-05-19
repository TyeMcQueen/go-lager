package grpc_lager

import (
	"context"
	"time"

	"github.com/TyeMcQueen/go-lager"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
	"google.golang.org/grpc/codes"
)

var (
	defaultOptions = &options{
		levelFunc:       DefaultCodeToLevel,
		shouldLog:       grpc_logging.DefaultDeciderMethod,
		codeFunc:        grpc_logging.DefaultErrorToCode,
		durationFunc:    DefaultDurationToField,
		messageFunc:     DefaultMessageProducer,
		timestampFormat: time.RFC3339,
	}
)

type options struct {
	levelFunc       CodeToLevel
	shouldLog       grpc_logging.Decider
	codeFunc        grpc_logging.ErrorToCode
	durationFunc    DurationToField
	messageFunc     MessageProducer
	timestampFormat string
}

func evaluateServerOpt(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	optCopy.levelFunc = DefaultCodeToLevel
	for _, o := range opts {
		o(optCopy)
	}

	return optCopy
}

type Option func(*options)

// CodeToLevel function defines the mapping between gRPC return codes and interceptor log level.
type CodeToLevel func(code codes.Code) byte

// DurationToField function defines how to produce duration fields for logging
type DurationToField func(duration time.Duration) lager.AMap

// WithDecider customizes the function for deciding if the gRPC interceptor logs should log.
func WithDecider(f grpc_logging.Decider) Option {
	return func(o *options) {
		o.shouldLog = f
	}
}

// WithLevels customizes the function for mapping gRPC return codes and interceptor log level statements.
func WithLevels(f CodeToLevel) Option {
	return func(o *options) {
		o.levelFunc = f
	}
}

// WithCodes customizes the function for mapping errors to error codes.
func WithCodes(f grpc_logging.ErrorToCode) Option {
	return func(o *options) {
		o.codeFunc = f
	}
}

// WithDurationField customizes the function for mapping request durations to Zap fields.
func WithDurationField(f DurationToField) Option {
	return func(o *options) {
		o.durationFunc = f
	}
}

// WithMessageProducer customizes the function for message formation.
func WithMessageProducer(f MessageProducer) Option {
	return func(o *options) {
		o.messageFunc = f
	}
}

// WithTimestampFormat customizes the timestamps emitted in the log fields.
func WithTimestampFormat(format string) Option {
	return func(o *options) {
		o.timestampFormat = format
	}
}

// DefaultCodeToLevel is the default implementation of gRPC return codes and interceptor log level for server side.
func DefaultCodeToLevel(code codes.Code) byte {
	switch code {
	case codes.OK:
		return 'I'
	case codes.Canceled:
		return 'I'
	case codes.Unknown:
		return 'E'
	case codes.InvalidArgument:
		return 'I'
	case codes.DeadlineExceeded:
		return 'W'
	case codes.NotFound:
		return 'I'
	case codes.AlreadyExists:
		return 'I'
	case codes.PermissionDenied:
		return 'W'
	case codes.Unauthenticated:
		return 'I' // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return 'W'
	case codes.FailedPrecondition:
		return 'W'
	case codes.Aborted:
		return 'W'
	case codes.OutOfRange:
		return 'W'
	case codes.Unimplemented:
		return 'E'
	case codes.Internal:
		return 'E'
	case codes.Unavailable:
		return 'W'
	case codes.DataLoss:
		return 'E'
	default:
		return 'E'
	}
}

// DefaultDurationToField is the default implementation of converting request duration to a Zap field.
var DefaultDurationToField = DurationToTimeMillisField

// DurationToTimeMillisField converts the duration to milliseconds and uses the key `grpc.time_ms`.
func DurationToTimeMillisField(duration time.Duration) lager.AMap {
	return lager.Pairs("grpc.time_ms", durationToMilliseconds(duration))
}

// DurationToDurationField uses a Duration field to log the request duration
// and leaves it up to Zap's encoder settings to determine how that is output.
func DurationToDurationField(duration time.Duration) lager.AMap {
	return lager.Pairs("grpc.duration", duration)
}

func durationToMilliseconds(duration time.Duration) float32 {
	return float32(duration.Nanoseconds()/1000) / 1000
}

// MessageProducer produces a user defined log message
type MessageProducer func(ctx context.Context, msg string, level byte, code codes.Code, err error, duration lager.AMap)

// DefaultMessageProducer writes the default message
func DefaultMessageProducer(ctx context.Context, msg string, level byte, code codes.Code, err error, duration lager.AMap) {
	// ctx = lager.ContextPairs(ctx).Merge(duration).InContext(ctx)
	lager.Level(level, ctx).MMap(msg,
		"grpc.code", code,
		lager.Unless(nil == err, "error"), err,
	)
}
