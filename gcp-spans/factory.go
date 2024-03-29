/*
	This package is still in beta and the public interface may undergo changes
	without a full deprecation cycle.
*/
package spans

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const TraceHeader = "X-Cloud-Trace-Context"

// HexChars is a 256-bit value that has a 1 bit at the offset of the ASCII
// values of '0'..'9', 'a'..'f', and 'A'..'F', the hexidecimal digits.
//
var HexChars = [8]uint32{0, 0x3ff0000, 0x7e, 0x7e, 0, 0, 0, 0}

// Used to store data in a Context:
type inContext string

const _contextSpan = inContext("span")

// ROSpan implements Factory but only deals with Import()ed spans, thus
// requiring no access to GCP CloudTrace libraries.  Such spans are
// read-only (hence "RO"), only dealing with spans created elsewhere
// and no changes can be made to them.
//
// The only non-trivial methods for ROSpan are Import() and ImportFromHeader()
// and then, on such imported spans: GetSpanID(), GetTraceID(),
// GetTracePath(), GetSpanPath(), GetCloudContext(), and SetHeader().
//
// NewSubSpan() always returns 'nil'.  The other New*() methods always
// return an empty span.  Methods that should log when called on an empty
// span also do not log for ROSpans since those methods do nothing even
// if the span is not empty.
//
type ROSpan struct {
	proj    string
	traceID string
	spanID  uint64
}

// Factory is an interface that allows Spans to be created and manipulated
// without depending on the GCP CloudTrace module (and its large list of
// dependencies).
//
// Each Factory instance can hold a single span or be empty.
//
type Factory interface {

	// GetProjectID() returns the GCP Project ID (which is not the Project
	// Number) for which spans will be registered.
	//
	GetProjectID() string

	// GetTraceID() returns "" if the Factory is empty.  Otherwise it returns
	// the trace ID of the contained span (which will not be "" nor a
	// hexadecimal representation of 0).
	//
	GetTraceID() string

	// GetSpanID() returns 0 if the Factory is empty.  Otherwise it returns
	// the span ID of the contained span (which will not be 0).
	//
	GetSpanID() uint64

	// GetStart() returns the time at which the span began.  Returns a zero
	// time if the Factory is empty or the contained span was Import()ed.
	//
	GetStart() time.Time

	// GetDuration() returns a negative duration if the factory is empty or
	// if the span has not been Finish()ed yet.  Otherwise, it returns the
	// span's duration.
	//
	GetDuration() time.Duration

	// GetTracePath() returns "" if the Factory is empty.  Otherwise it
	// returns the trace's resource sub-path which will be in the form
	// "projects/{projectID}/traces/{traceID}".
	//
	GetTracePath() string

	// GetSpanPath() returns "" if the Factory is empty.  Otherwise it
	// returns the span's resource sub-path which will be in the form
	// "traces/{traceID}/spans/{spanID}" where both IDs are in hexadecimal.
	//
	GetSpanPath() string

	// GetCloudContext() returns "" if the Factory is empty.  Otherwise it
	// returns a value appropriate for the "X-Cloud-Trace-Context:" header
	// which will be in the form "{traceID}/{spanID}" where spanID is in
	// base 10.
	//
	GetCloudContext() string

	// Import() returns a new Factory containing a span created somewhere
	// else.  If the traceID or spanID is invalid, then a 'nil' Factory and
	// an error are returned.  The usual reason to do this is so that you can
	// then call NewSubSpan().
	//
	Import(traceID string, spanID uint64) (Factory, error)

	// ImportFromHeaders() returns a new Factory containing a span created
	// somewhere else based on the "X-Cloud-Trace-Context:" header.  If the
	// header does not contain a valid CloudContext value, then a valid but
	// empty Factory is returned.
	//
	ImportFromHeaders(headers http.Header) Factory

	// SetHeader() sets the "X-Cloud-Trace-Context:" header if the Factory
	// is not empty.  Always returns the calling Factory so that further
	// method calls can be chained.
	//
	SetHeader(headers http.Header) Factory

	// NewTrace() returns a new Factory holding a new span, part of a new
	// trace.  Any span held in the invoking Factory is ignored.
	//
	NewTrace() Factory

	// NewSubSpan() returns a new Factory holding a new span that is a
	// sub-span of the span contained in the invoking Factory.  If the
	// invoking Factory was empty, then a failure with a stack trace is
	// logged and a 'nil' Factory is returned.
	//
	NewSubSpan() Factory

	// NewSpan() returns a new Factory holding a new span.  It does
	// NewTrace() if the Factory was empty and NewSubSpan() otherwise.
	//
	NewSpan() Factory

	// Sets the span kind to "SERVER".  Does nothing except log a failure
	// with a stack trace if the Factory is empty.  Always returns the calling
	// Factory so further method calls can be chained.
	//
	SetIsServer() Factory

	// Sets the span kind to "CLIENT".  Does nothing except log a failure
	// with a stack trace if the Factory is empty.  Always returns the
	// calling Factory so further method calls can be chained.
	//
	SetIsClient() Factory

	// Sets the span kind to "PRODUCER" (the term used for a process that
	// asynchronously publishes data like with pub/sub).  Does nothing
	// except log a failure with a stack trace if the Factory is empty.
	// Always returns the calling Factory so further method calls can be
	// chained.
	//
	SetIsPublisher() Factory

	// Sets the span kind to "CONSUMER" (the term used for a process that
	// asynchronously receives data like a pub/sub subscriber).  Does nothing
	// except log a failure with a stack trace if the Factory is empty.
	// Always returns the calling Factory so further method calls can be
	// chained.
	//
	SetIsSubscriber() Factory

	// SetDisplayName() sets the display name on the contained span.  Does
	// nothing except log a failure with a stack trace if the Factory is
	// empty.  Always returns the calling Factory so further method calls
	// can be chained.
	//
	SetDisplayName(desc string) Factory

	// AddAttribute() adds an attribute key/value pair to the contained span.
	// Does nothing except log a failure with a stack trace if the Factory is
	// empty (even returning a 'nil' error).
	//
	// 'val' can be a 'string', an 'int' or 'int64', or a 'bool'.  If 'key'
	// is empty or 'val' is not one of the listed types, then an error is
	// returned and the attribute is not added.
	//
	AddAttribute(key string, val interface{}) error

	// AddPairs() takes a list of attribute key/value pairs.  For each pair,
	// AddAttribute() is called and any returned error is logged (including
	// a reference to the line of code that called AddPairs).  Always returns
	// the calling Factory so further method calls can be chained.
	//
	AddPairs(pairs ...interface{}) Factory

	// SetStatusCode() sets the status code on the contained span.
	// 'code' is expected to be a value from
	// google.golang.org/genproto/googleapis/rpc/code but this is not
	// verified.  HTTP status codes are also understood by the library.
	// Does nothing except log a failure with a stack trace if the Factory
	// is empty.  Always returns the calling span so further method calls
	// can be chained.
	//
	SetStatusCode(code int64) Factory

	// SetStatusMessage() sets the status message string on the contained
	// span.  By convention, only a failure should set a status message.
	// Does nothing except log a failure with a stack trace if the Factory
	// is empty.  Always returns the calling Factory so further method calls
	// can be chained.
	//
	SetStatusMessage(msg string) Factory

	// Finish() notifies the Factory that the contained span is finished.
	// The Factory will be empty afterward.  The Factory will arrange for the
	// span to be registered.
	//
	// The returned value is the duration of the span's life.  If the Factory
	// was already empty or the contained span was from Import(), then a
	// failure with a stack trace is logged and a 0 duration is returned.
	//
	Finish() time.Duration
}

// ContextStoreSpan() adds a span Factory to the passed-in Context,
// returning the new, decorated Context.
//
func ContextStoreSpan(ctx context.Context, span Factory) context.Context {
	return context.WithValue(ctx, _contextSpan, span)
}

// ContextGetSpan() extracts a span Factory from the passed-in Context and
// returns it (or 'nil').
//
func ContextGetSpan(ctx context.Context) Factory {
	if ix := ctx.Value(_contextSpan); nil != ix {
		return ix.(Factory)
	}
	return nil
}

// NonHexIndex() returns the offset to the first character in the string that
// is not a hexadecimal digit (0..9, a..f, A..F) or -1 if none.
//
func NonHexIndex(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 0 == HexChars[c>>5]&(1<<uint(c&31)) {
			return i
		}
	}
	return -1
}

// IsValidTraceID() returns 'true' only if 's' is a non-zero hexadecimal
// value of 32 digits.
//
func IsValidTraceID(s string) bool {
	if 32 != len(s) || -1 != NonHexIndex(s) || s == "00000000000000000000000000000000" {
		return false
	}
	return true
}

func HexSpanID(spanID uint64) string {
	return fmt.Sprintf("%016x", spanID)
}

// FinishSpan() calls Finish() on the passed-in 'span', unless it is 'nil'.
// It is most useful with 'defer' when a 'span' might be 'nil':
//
//      defer spans.FinishSpan(span)
//
func FinishSpan(span Factory) time.Duration {
	if nil != span && 0 != span.GetSpanID() && !span.GetStart().IsZero() {
		return span.Finish()
	}
	return time.Duration(0)
}

// NewROSpan() returns an empty Factory.
func NewROSpan(projectID string) ROSpan {
	return ROSpan{proj: projectID}
}

// SetSpanID() lets you set the spanID to make implementing a non-read-only
// span type easier.  This is the only method that requires a '*ROSpan' not
// just a 'ROSpan'.
//
func (s *ROSpan) SetSpanID(spanID uint64) {
	s.spanID = spanID
}

// GetProjectID() returns the GCP Project ID.
func (s ROSpan) GetProjectID() string {
	return s.proj
}

func (s ROSpan) GetTraceID() string {
	return s.traceID
}

func (s ROSpan) GetSpanID() uint64 {
	return s.spanID
}

func (s ROSpan) GetStart() time.Time {
	return time.Time{}
}

func (s ROSpan) GetDuration() time.Duration {
	return -time.Second
}

// GetTracePath() "projects/{projectID}/traces/{traceID}" or "".
func (s ROSpan) GetTracePath() string {
	if 0 == s.spanID {
		return ""
	}
	return "projects/" + s.proj + "/traces/" + s.traceID
}

// GetSpanPath() returns "traces/{traceID}/spans/{spanID}" or "".
func (s ROSpan) GetSpanPath() string {
	if 0 == s.spanID {
		return ""
	}
	return "traces/" + s.traceID + "/spans/" + HexSpanID(s.spanID)
}

// GetCloudContext() returns "{hex:traceID}/{decimal:spanID}" or "".
func (s ROSpan) GetCloudContext() string {
	if 0 == s.spanID {
		return ""
	}
	return s.traceID + "/" + strconv.FormatUint(s.spanID, 10)
}

// Import() returns a new Factory containing a span created elsewhere.
func (s ROSpan) Import(traceID string, spanID uint64) (Factory, error) {
	if 0 == spanID {
		return nil, fmt.Errorf("Import(): Span ID of 0 not allowed")
	} else if 32 != len(traceID) {
		return nil, fmt.Errorf(
			"Import(): Invalid trace ID has length %d not 32 (%s)",
			len(traceID), traceID)
	} else if i := NonHexIndex(traceID); -1 != i {
		return nil, fmt.Errorf(
			"Import(): Invalid trace ID (%s) has non-hex char (%c)",
			traceID, traceID[i])
	} else if traceID == "00000000000000000000000000000000" {
		return nil, fmt.Errorf("Import(): Trace ID of 32 '0's not allowed")
	}
	return ROSpan{proj: s.proj, spanID: spanID, traceID: traceID}, nil
}

func (s ROSpan) ImportFromHeaders(headers http.Header) Factory {
	parts := strings.Split(headers.Get(TraceHeader), "/")
	if 2 == len(parts) {
		spanID, _ := strconv.ParseUint(parts[1], 10, 64)
		if im, _ := s.Import(parts[0], spanID); nil != im {
			return im
		}
	}
	return ROSpan{proj: s.proj}
}

func (s ROSpan) SetHeader(headers http.Header) Factory {
	if 0 != s.spanID {
		headers.Set(TraceHeader, s.GetCloudContext())
	}
	return s
}

func (s ROSpan) SetIsServer() Factory              { return s }
func (s ROSpan) SetIsClient() Factory              { return s }
func (s ROSpan) SetIsPublisher() Factory           { return s }
func (s ROSpan) SetIsSubscriber() Factory          { return s }
func (s ROSpan) SetDisplayName(_ string) Factory   { return s }
func (s ROSpan) SetStatusCode(_ int64) Factory     { return s }
func (s ROSpan) SetStatusMessage(_ string) Factory { return s }

func (s ROSpan) NewTrace() Factory {
	return ROSpan{proj: s.proj}
}

func (s ROSpan) NewSubSpan() Factory {
	return nil
}

func (s ROSpan) NewSpan() Factory {
	return ROSpan{proj: s.proj}
}

func (s ROSpan) AddAttribute(_ string, _ interface{}) error {
	return nil
}

func (s ROSpan) AddPairs(_ ...interface{}) Factory {
	return s
}

func (s ROSpan) Finish() time.Duration {
	return time.Duration(0)
}
