package lager

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/TyeMcQueen/go-lager/gcp-spans"
)

const GcpSpanKey = "logging.googleapis.com/spanId"
const GcpTraceKey = "logging.googleapis.com/trace"

const projIdUrl = "http://metadata.google.internal/computeMetadata/v1/project/project-id"

var projectID string

// GcpProjectID() returns the current GCP project ID [which is not the
// project number].  Once the lookup succeeds, that value is saved and
// returned for subsequent calls.  The lookup times out after 0.1s.
//
// Set GCP_PROJECT_ID in your environment to avoid the more complex lookup.
//
func GcpProjectID(ctx Ctx) (string, error) {
	if "" == projectID {
		projectID = os.Getenv("GCP_PROJECT_ID")
	}
	if "" == projectID {
		if nil == ctx {
			ctx = context.Background()
		}
		reqCtx, can := context.WithTimeout(ctx, 100*time.Millisecond)
		defer can()
		req, err := http.NewRequestWithContext(reqCtx, "GET", projIdUrl, nil)
		if nil != err {
			return "", fmt.Errorf("GcpProjectID() is broken: %w", err)
		}
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := new(http.Client).Do(req)
		if nil != err {
			return "", fmt.Errorf("Can't get GCP project ID (from %s): %w",
				projIdUrl, err)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if nil != err {
			return "", fmt.Errorf(
				"Can't read GCP project ID from response body: %w", err)
		}
		projectID = string(b)
	}
	return projectID, nil
}

// RunningInGcp() tells Lager to log messages in a format that works best
// when running inside of the Google Cloud Platform (when using GCP Cloud
// Logging).  You can call this so you don't have to set LAGER_GCP=1 in your
// environment, but note that it will be possible for logging to happen
// before this call is executed [such as some logging triggered, perhaps
// indirectly, by some code in an Init() function] and such logs would not
// be in the desired format.  Even calling RunningInGcp() from your own
// Init() function will not guarantee it happens before any logging.  For
// this reason, using LAGER_GCP=1 is preferred.
//
// In particular, RunningInGcp() is equivalent to running:
//
//      if "" == os.Getenv("LAGER_KEYS") {
//          // LAGER_KEYS has precedence over LAGER_GCP.
//          lager.Keys("time", "severity", "message", "data", "", "module")
//      }
//      lager.SetLevelNotation(lager.GcpLevelName)
//
// It also arranges for an extra element to be added to the JSON if nothing
// but a message is logged so that jsonPayload.message does not get
// transformed into textPayload when the log is ingested into Cloud Logging.
//
func RunningInGcp() {
	updateGlobals(setRunningInGcp(true))
}

// How GCP options are set safely.
func setRunningInGcp(enabled bool) func(*globals) {
	return func(g *globals) {
		g.inGcp = enabled
		if enabled {
			if "" == os.Getenv("LAGER_KEYS") {
				g.keys = &keyStrs{
					when: "time", lev: "severity", msg: "message",
					args: "data", mod: "module", ctx: "",
				}
			}
			// TODO: g.levDesc = GcpLevelName
			SetLevelNotation(GcpLevelName)
		} else {
			// TODO: g.levDesc = identLevelNotation
			SetLevelNotation(nil)
		}
	}
}

// GcpLevelName takes a Lager level name (only the first letter matters and
// it must be upper case) and returns the corresponding value GCP uses in
// structured logging to represent the severity of such logs.  Levels are
// mapped as:
//      Not used: Alert ("700") and Emergency ("800")
//      Panic, Exit - Critical ("600")
//      Fail - Error ("500")
//      Warn - Warning ("400")
//      Note - Notice ("300")
//      Access, Info - Info ("200")
//      Trace, Debug, Obj, Guts - Debug ("100")
//      If an invalid level name is passed: Default ("0")
//
// If the environment variable LAGER_GCP is not empty, then
// lager.LevelNotation will be initalized to lager.GcpLevelName.
//
func GcpLevelName(lev string) string {
	switch lev[0] {
	case 'P', 'E':
		// We could import "cloud.google.com/go/logging" to get these
		// constants, but that pulls in hundreds of dependencies which
		// is not a reasonable trade-off for getting 7 constants.
		return "600"
	case 'F':
		return "500"
	case 'W':
		return "400"
	case 'N':
		return "300"
	case 'A', 'I':
		return "200"
	case 'T', 'D', 'O', 'G':
		return "100"
	}
	return "0"
}

// GcpFakeResponse() creates an http.Response suitable for passing to
// GcpHttp() [or similar] when you just have a status code (and/or a
// response size) and not a http.Response.
//
// Pass 'size' as -1 to omit this information.  Passing 'status' as 0 will
// cause an explicit 0 to be logged.  Pass 'status' as -1 to omit it.
// Pass 'desc' as "" to have it set based on 'status'.
//
func GcpFakeResponse(status int, size int64, desc string) *http.Response {
	if "" == desc {
		desc = http.StatusText(status)
	}
	return &http.Response{
		Status:        desc,
		StatusCode:    status,
		ContentLength: size,
	}
}

// RequestUrl() returns a *url.URL more appropriate for logging, based on
// an *http.Request.  For server Requests, the missing Host and Scheme are
// populated.
//
func RequestUrl(req *http.Request) *url.URL {
	uri := *req.URL
	if "" == uri.Host {
		uri.Host = req.Host
	}
	uri.Scheme = "http"
	if fp := req.Header.Get("X-Forwarded-Proto"); "" != fp {
		uri.Scheme = fp
	} else if nil != req.TLS {
		uri.Scheme = "https"
	}
	return &uri
}

// GcpHtttp() returns a value for logging that GCP will recognize as details
// about an HTTP(S) request (and perhaps its response), if placed under the
// key "httpRequest".
//
// 'req' must not be 'nil' but 'resp' and 'start' can be.  None of the
// arguments passed will be modified; 'start' is of type '*time.Time' only
// to make it simple to omit latency calculations by passing in 'nil'.
// If 'start' points to a 'time.Time' that .IsZero(), then it is ignored.
//
// When using tracing, this allows GCP logging to display log lines for the
// same request (if each includes this block) together.  So this can be a
// good thing to add to a context.Context used with your logging.  For this
// to work, you must log a final message that includes all three arguments
// (as well as using GCP-compatible tracing).
//
// The following items will be logged (in order in the original JSON, but
// GCP does not preserve order of JSON keys, understandably), except that
// some can be omitted depending on what you pass in.
//
//      "requestMethod"     E.g. "GET"
//      "requestUrl"        E.g. "https://cool.me/api/v1"
//      "protocol"          E.g. "HTTP/1.1"
//      "status"            E.g. 403
//      "requestSize"       Omitted if the request body size is not yet known.
//      "responseSize"      Omitted if 'resp' is 'nil' or body size not known.
//      "latency"           E.g. "0.1270s".  Omitted if 'start' is 'nil'.
//      "remoteIp"          E.g. "127.0.0.1"
//      "serverIp"          Not currently ever included.
//      "referer"           Omitted if there is no Referer[sic] header.
//      "userAgent"         Omitted if there is no User-Agent header.
//
// Note that "status" is logged as "0" in the special case where 'resp' is
// 'nil' but 'start' is not 'nil'.  This allows you to make an "access log"
// entry for cases where you got an error that prevents you from either
// making or getting an http.Response.
//
// See also GcpHttpF() and GcpRequestAddTrace().
//
func GcpHttp(req *http.Request, resp *http.Response, start *time.Time) RawMap {
	ua := req.Header.Get("User-Agent")
	ref := req.Header.Get("Referer")
	reqSize := req.ContentLength

	remoteAddr := req.RemoteAddr
	if remoteIp, _, err := net.SplitHostPort(remoteAddr); nil == err {
		remoteAddr = remoteIp
	}
	// TODO: Add support for proxy headers?
	//  if ... req.Header.Get("X-Forwarded-For") {
	//      remoteIp = ...
	//  }

	if nil != start && (*start).IsZero() {
		start = nil
	}
	status := -1
	respSize := int64(-1)
	if nil != resp {
		status = resp.StatusCode
		respSize = resp.ContentLength
	} else if nil != start {
		status = 0
	}

	lag := ""
	if nil != start {
		lag = fmt.Sprintf("%.4fs", time.Now().Sub(*start).Seconds())
	}

	uri := RequestUrl(req)

	return Map(
		"requestMethod", req.Method,
		"requestUrl", uri.String(),
		"protocol", req.Proto,
		Unless(-1 == status, "status"), status,
		Unless(reqSize < 0, "requestSize"), reqSize,
		Unless(respSize < 0, "responseSize"), respSize,
		Unless("" == lag, "latency"), lag,
		"remoteIp", remoteAddr,
		// "serverIp", ?,
		Unless("" == ref, "referer"), ref,
		Unless("" == ua, "userAgent"), ua,
	)
}

// GcpHttpF() can be used for logging just like GcpHttp(), it just returns a
// function so that the work is only performed if the logging level is enabled.
//
// If you are including GcpHttp() information in a lot of log lines [which can
// be quite useful], then you can get even more efficiency by adding the pair
// ' "httpRequest", GcpHttp(req, nil, nil) ' to your Context [which you then
// pass to 'lager.Warn(ctx)', for example] so the data is only calculated
// once.  You can add this to an *http.Request's Context by calling
// GcpRequestAddTrace().
//
// For this to work best, you should specify "" as the key name for context
// information; which is automatically done if LAGER_GCP is non-empty in the
// environment and LAGER_KEYS is not set.
//
// You'll likely include 'GcpHttp(req, resp, &start)' in one log line [to
// record the response information and latency, not just the request].  If
// you added "httpRequest" to your context, then that logging is best done
// via:
//
//      lager.Acc(
//          lager.AddPairs(ctx, "httpRequest", GcpHttp(req, resp, &start)),
//      ).List("Response sent")
//
// so that the request-only information is not also output.  Doing this via
// GcpLogAccess() is easier.
//
func GcpHttpF(
	req *http.Request, resp *http.Response, start *time.Time,
) func() interface{} {
	return func() interface{} {
		return GcpHttp(req, resp, start)
	}
}

// GcpLogAccess() creates a standard "access log" entry.  It is just a handy
// shortcut for:
//
//      lager.Acc(
//          lager.AddPairs(req.Context(),
//              "httpRequest", GcpHttp(req, resp, pStart)))
//
// You would use it like, for example:
//
//      lager.GcpLogAccess(req, resp, &start).MMap(
//          "Response sent", "User", userID)
//
func GcpLogAccess(
	req *http.Request, resp *http.Response, pStart *time.Time,
) Lager {
	return Acc(
		AddPairs(req.Context(), "httpRequest", GcpHttp(req, resp, pStart)))
}

// GcpContextAddTrace() takes a Context and returns one that has the span
// added as 2 pairs that will be logged and recognized by GCP when that
// Context is passed to lager.Warn() or similar methods.  If 'span' is 'nil'
// or an empty Factory, then the original 'ctx' is just returned.
//
// 'ctx' is the Context from which the new Context is created.  'span'
// contains the GCP CloudTrace span to be added.
//
// See also GcpContextReceivedRequest() and/or GcpContextSendingRequest()
// which call this and do several other useful things.
//
func GcpContextAddTrace(ctx Ctx, span spans.Factory) Ctx {
	if nil != span && 0 != span.GetSpanID() {
		ctx = AddPairs(ctx,
			GcpTraceKey, span.GetTracePath(),
			GcpSpanKey, spans.HexSpanID(span.GetSpanID()))
	}
	return ctx
}

// GcpContextReceivedRequest() does several things that are useful when
// a server receives a new request.  'ctx' is the Context passed to the
// request handler and 'req' is the received request.
//
// An "httpRequest" key/value pair is added to the Context so that the
// request details will be included in any subsequent log lines [when the
// returned Context is passed to lager.Warn() or similar methods].
//
// If the request headers include GCP trace information, then that is
// extracted [see spans.Factory.ImportFromHeaders()].
//
// If 'ctx' contains a spans.Factory, then that is fetched and used to
// create either a new sub-span or (if there is no CloudTrace context in
// the headers) a new trace (and span).  If the Factory is able to create
// a new span, then it is marked as a "SERVER" span, its Display Name is
// set to GetSpanPrefix() + ".in.request", and it is stored in the context
// via spans.ContextStoreSpan().
//
// If a span was imported or created, then the span information is added
// to the Context as pairs to be logged [see GcpContextAddTrace()] and
// a span will be contained in the returned Factory.
//
// The updated Context is returned (Contexts are immutable).
//
// It is usually called in a manner similar to:
//
//      ctx, span := lager.GcpContextReceivedRequest(ctx, req)
//      defer spans.FinishSpan(span)
// or
//      ctx, span := lager.GcpContextReceivedRequest(ctx, req)
//      var resp *http.Response
//      defer lager.GcpSendingResponse(span, req, resp)
//
// See also GcpReceivedRequest().
//
// The order of arguments is 'ctx' then 'req' as information moves only in
// the direction ctx <- req (if we consider 'ctx' to represent both the
// argument and the returned value) and information always moves right-to-
// left in Go (in assignment statements and when using channels).
//
func GcpContextReceivedRequest(
	ctx Ctx, req *http.Request,
) (Ctx, spans.Factory) {
	ctx = AddPairs(ctx, "httpRequest", GcpHttp(req, nil, nil))
	span := spans.ContextGetSpan(ctx)
	if nil == span {
		if proj, err := GcpProjectID(nil); nil != err {
			Fail(ctx).MMap("Could not get GCP Project ID", "err", err)
		} else { // Can't write new spans; just do read-only span operations:
			span = spans.NewROSpan(proj)
		}
	}
	if nil != span {
		span = span.ImportFromHeaders(req.Header)
		if sub := span.NewSpan(); nil != sub {
			span = sub
			span.SetDisplayName(GetSpanPrefix() + ".in.request")
			span.SetIsServer()
			ctx = spans.ContextStoreSpan(ctx, span)
		}
		ctx = GcpContextAddTrace(ctx, span)
	}
	return ctx, span
}

// GcpReceivedRequest() gets the Context from '*pReq' and uses it to call
// GcpContextReceivedRequest().  Then it replaces '*pReq' with a version of
// the request with the new Context attached.  Then it returns the Factory.
//
// It is usually called in a manner similar to:
//
//      defer spans.FinishSpan(lager.GcpReceivedRequest(&req))
// or
//      var resp *http.Response // Will be set in later code
//      defer lager.GcpSendingResponse(
//          lager.GcpReceivedRequest(&req), req, resp)
//
// Using GcpContextReceivedRequest() can be slightly more efficient if you
// either start with a Context different from the one attached to the
// Request or will not attach the new Context to the Request (or will adjust
// it further before attaching it) since each time WithContext() is called
// on a Request, the Request must be copied.
//
func GcpReceivedRequest(pReq **http.Request) spans.Factory {
	ctx, span := GcpContextReceivedRequest((*pReq).Context(), *pReq)
	*pReq = (*pReq).WithContext(ctx)
	return span
}

// GcpContextSendingRequest() does several things that are useful when a
// server is about to send a request to a dependent service.  'req' is the
// Request that is about to be sent.  'ctx' is the server's current Context.
//
// The current span is fetched from 'ctx' [such as the one placed there
// by GcpReceivedRequest() when the original request was received].  A new
// sub-span is created, if possible.  If so, then it is marked as a "CLIENT"
// span, its Display Name is set to GetSpanPrefix() + ".out.request", it is
// stored in the Context via spans.ContextStoreSpan(), the returned Factory
// will contain the new span, and the updated Context will contain 2 pairs
// (to be logged) from the new span.  Note that the original Context is not
// (cannot be) modified, so the trace/span pair logged after the
// request-sending function returns will revert to the prior span.
//
// If a span was found or created, then its CloudContext is added to the
// headers for 'req' so that the dependent service can log it and add its
// own spans to the trace (unless 'req' is 'nil').
//
// The updated Context is returned (Contexts are immutable).
//
// The order of arguments is 'req' then 'ctx' as information moves only
// in the direction req <- ctx and information always moves right-to-left
// in Go (in assignment statements and when using channels).
//
// It is usually called in a manner similar to:
//
//      ctx, span := lager.GcpContextSendingRequest(req, ctx)
//      defer spans.FinishSpan(span)
//
// See also GcpSendingRequest().
//
func GcpContextSendingRequest(
	req *http.Request, ctx Ctx,
) (Ctx, spans.Factory) {
	span := spans.ContextGetSpan(ctx)
	if nil != span {
		subspan := span.NewSpan()
		if nil != subspan {
			span = subspan
			span.SetDisplayName(GetSpanPrefix() + ".out.request")
			span.SetIsClient()
			ctx = spans.ContextStoreSpan(ctx, span)
			ctx = GcpContextAddTrace(ctx, span)
		}
		if nil != req {
			span.SetHeader(req.Header)
		}
	}
	return ctx, span
}

// GcpSendingNewRequest() does several things that are useful when a
// server is about to send a request to a dependent service, by calling
// GcpContextSendingRequest().  It takes the same arguments as
// http.NewRequestWithContext() but returns an extra value.
//
// It is usually called in a manner similar to:
//
//      req, span, err := lager.GcpSendingNewRequest(ctx, "GET", url, nil)
//      if nil != err { ... }
//      defer spans.FinishSpan(span)
//
func GcpSendingNewRequest(
	ctx Ctx, method, url string, body io.Reader,
) (*http.Request, spans.Factory, error) {
	ctx, span := GcpContextSendingRequest(nil, ctx)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if nil != err {
		// ('span' will just get garbage collected and not registered.)
		return nil, nil, err
	}
	if nil != span {
		span.SetHeader(req.Header)
	}
	return req, span, nil
}

// GcpSendingRequest() does several things that are useful when a
// server is about to send a request to a dependent service, by calling
// GcpContextSendingRequest().  It uses the Context from '*pReq' and then
// replaces '*pReq' with a copy of the original Request but with the new
// Context attached.
//
// It is usually called in a manner similar to:
//
//      defer spans.FinishSpan(lager.GcpSendingRequest(&req))
//
func GcpSendingRequest(pReq **http.Request) spans.Factory {
	ctx, span := GcpContextSendingRequest(*pReq, (*pReq).Context())
	*pReq = (*pReq).WithContext(ctx)
	return span
}

// GcpFinishSpan() updates a span with the status information from a
// http.Response and Finish()es the span (which registers it with GCP).
//
func GcpFinishSpan(span spans.Factory, resp *http.Response) time.Duration {
	if nil == span || span.GetStart().IsZero() {
		return time.Duration(0)
	}
	span.SetStatusCode(int64(resp.StatusCode))
	if "" != resp.Status {
		span.SetStatusMessage(resp.Status)
	}
	return span.Finish()
}

// GcpSendingResponse() does several things that are useful when a server
// is about to send a response to a request it received.  It combines
// GcpLogAccess() and GcpFinishSpan().  The access log line written will
// use the message "Sending response" and will include the passed-in 'pairs'
// which should be zero or more pairs of a string key followed by an
// arbitrary value.
//
// 'resp' will often be constructed via GcpFakeResponse().
//
func GcpSendingResponse(
	span spans.Factory,
	req *http.Request,
	resp *http.Response,
	pairs ...interface{},
) {
	var pStart *time.Time
	if nil != span {
		start := span.GetStart()
		pStart = &start
	}
	GcpLogAccess(req, resp, pStart).MMap(
		"Sending response", InlinePairs, pairs)
	GcpFinishSpan(span, resp)
}

// GcpReceivedResponse() combines GcpLogAccess() and GcpFinishSpan().
// The access log line written will use the message "Received response"
// and will include the passed-in 'pairs' which should be zero or more
// pairs of a string key followed by an arbitrary value.  However, logging
// every response received from a dependent service may be excessive.
//
func GcpReceivedResponse(
	span spans.Factory,
	req *http.Request,
	resp *http.Response,
	pairs ...interface{},
) {
	var pStart *time.Time
	if nil != span {
		start := span.GetStart()
		pStart = &start
	}
	GcpLogAccess(req, resp, pStart).MMap(
		"Received response", InlinePairs, pairs)
	GcpFinishSpan(span, resp)
}
