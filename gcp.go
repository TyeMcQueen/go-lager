package lager

import (
	"fmt"
	"net/http"
	"time"
)

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

// GcpHtttp() returns a value for logging that GCP will recognize as details
// about an HTTP(S) request (and perhaps its response), if placed under the
// key "httpRequest".
//
// 'req' must not be 'nil' but 'resp' and 'start' can be.  '*start' will not
// be modified; 'start' is of type '*time.Time' only to make it simple to omit
// latency calculations by passing in 'nil'.
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
//      "remoteIp"          Client IP plus port, e.g. "127.0.0.1:12345"
//      "serverIp"          Not currently ever included.
//      "referer"           Omitted if there is no Referer[sic] header.
//      "userAgent"         Omitted if there is no User-Agent header.
//
// See also GcpHttpF().
//
func GcpHttp(req *http.Request, resp *http.Response, start *time.Time) RawMap {
	ua := req.Header.Get("User-Agent")
	ref := req.Header.Get("Referer")
	reqSize := req.ContentLength

	remoteAddr := req.RemoteAddr
	// TODO: Add support for proxy headers.
	//  if ... req.Header.Get("X-Forwarded-For") {
	//      remoteIp = ...
	//  }

	status := 0
	respSize := int64(-1)
	if nil != resp {
		status = resp.StatusCode
		respSize = resp.ContentLength
	}
	lag := ""
	if nil != start {
		lag = fmt.Sprintf("%.4fs", time.Now().Sub(*start).Seconds())
	}

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

	return Map(
		"requestMethod", req.Method,
		"requestUrl", uri.String(),
		"protocol", req.Proto,
		Unless(0 == status, "status"), status,
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
// once.
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
// so that the request-only information is not also output.  GcpLogAccess()
// makes this easy.
//
func GcpHttpF(
	req *http.Request, resp *http.Response, start *time.Time,
) (func() interface{}) {
	return func() interface{} {
		return GcpHttp(req, resp, start)
	}
}

// GcpLogAccess() is just a handy shortcut for:
//
//      lager.Acc(
//          lager.AddPairs(req.Context(),
//              "httpRequest", GcpHttp(req, resp, pStart)))
//
// You would use it like, for example:
//
//      lager.GcpLogAccess(req, resp, &start).List("Response sent")
//
func GcpLogAccess(
	req *http.Request, resp *http.Response, pStart *time.Time,
) Lager {
	return Acc(
		AddPairs(req.Context(), "httpRequest", GcpHttp(req, resp, pStart)))
}
