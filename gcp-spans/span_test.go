package spans_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/TyeMcQueen/go-lager/gcp-spans"
	"github.com/TyeMcQueen/go-tutl"
)

type TestSpan struct {
	spans.ROSpan
	finishes int
}

func (ts TestSpan) GetSpanID() uint64 { return 20 }
func (ts TestSpan) GetStart() time.Time { return time.Now() }
func (ts *TestSpan) Finish() time.Duration { ts.finishes++; return 0 }

func TestSpans(t *testing.T) {
	u := tutl.New(t)

	proj := "my-gcp-project-id"
	empty := spans.Factory(spans.NewROSpan(proj))

	u.Is(proj, empty.GetProjectID(), "empty GetProjectID")
	u.Is("", empty.GetTraceID(), "empty GetTraceID")
	u.Is(0, empty.GetSpanID(), "empty GetSpanID")
	u.Is(time.Time{}, empty.GetStart(), "empty GetStart")
	u.Is("", empty.GetTracePath(), "empty GetTracePath")
	u.Is("", empty.GetSpanPath(), "empty GetSpanPath")
	u.Is("", empty.GetCloudContext(), "empty GetCloudContext")

	fakeHeader := make(http.Header)
	empty.SetHeader(fakeHeader)
	u.Is(0, len(fakeHeader), "empty SetHeader is no-op")

	u.Is(empty, empty.NewTrace(), "empty NewTrace")
	u.Is(empty, empty.NewSpan(), "empty NewSpan")
	u.Is(nil, empty.NewSubSpan(), "empty NewSubSpan")
	u.Is(nil, empty.AddAttribute("key", "value"), "empty AddAttribute")
	u.Is(time.Duration(0), empty.Finish(), "empty Finish")

	ti := "00000000000000000000000000000001"
	sp, err := empty.Import(ti, 20)
	u.IsNot(nil, sp, "Import")
	u.Is(nil, err, "Import error")

	u.Is(proj, sp.GetProjectID(), "GetProjectID")
	u.Is(ti, sp.GetTraceID(), "GetTraceID")
	u.Is(20, sp.GetSpanID(), "GetSpanID")
	u.Is(time.Time{}, sp.GetStart(), "GetStart")
	u.Is("projects/"+proj+"/traces/"+ti, sp.GetTracePath(), "GetTracePath")
	u.Is("traces/"+ti+"/spans/0000000000000014",
		sp.GetSpanPath(), "GetSpanPath")
	u.Is(ti+"/20", sp.GetCloudContext(), "GetCloudContext")

	sp.SetHeader(fakeHeader)
	u.Is(ti+"/20", fakeHeader.Get(spans.TraceHeader),
		"SetHeader sets "+spans.TraceHeader)

	u.Is(empty, sp.NewTrace(), "NewTrace")
	u.Is(empty, sp.NewSpan(), "NewSpan")
	u.Is(nil, sp.NewSubSpan(), "NewSubSpan")
	u.Is(nil, sp.AddAttribute("key", "value"), "AddAttribute")
	u.Is(time.Duration(0), sp.Finish(), "Finish")

	sp2, err := sp.Import(ti, 0)
	u.Is(nil, sp2, "Import 0 span")
	u.Like(err, "Import 0 span err",
		"*span id", " 0 ", "*not allowed", "Import")

	sp2, err = sp.Import(ti+"1", 20)
	u.Is(nil, sp2, "Import long trace")
	u.Like(err, "Import long trace err",
		"*trace id", "* length ", " 33 ", "* not 32", "Import", ti+"1")

	sp2, err = sp.Import(ti[1:]+"x", 20)
	u.Is(nil, sp2, "Import non hex")
	u.Like(err, "Import non hex err",
		"*trace id", "*non-hex char", "[(]x[)]", "Import", ti[1:]+"x")

	sp2, err = sp.Import("00000000000000000000000000000000", 20)
	u.Is(nil, sp2, "Import zero trace")
	u.Like(err, "Import zero trace err",
		"*trace id", "'0'", "Import")

	sp = sp.ImportFromHeaders(fakeHeader)
	if u.IsNot(nil, sp, "ImportFromHeaders") {
		u.Is(ti, sp.GetTraceID(), "GetTraceID from headers")
		u.Is(20, sp.GetSpanID(), "GetSpanID from headers")
	}

	fakeHeader.Set(spans.TraceHeader, "no slash")
	sp = sp.ImportFromHeaders(fakeHeader)
	if u.IsNot(nil, sp.ImportFromHeaders(fakeHeader), "ImportFromHeaders no slash") {
		u.Is("", sp.GetTraceID(), "GetTraceID from headers no slash")
		u.Is(0, sp.GetSpanID(), "GetSpanID from headers no slash")
	}

	ro := spans.NewROSpan(proj)
	ro.SetSpanID(30)
	u.Is(30, ro.GetSpanID(), "SetSpanID() works")

	// Just verify these doesn't panic:
	spans.FinishSpan(nil)
	spans.FinishSpan(sp)
	sp.SetIsServer()
	sp.SetIsClient()
	sp.SetIsPublisher()
	sp.SetIsSubscriber()
	sp.SetDisplayName("")
	sp.SetStatusCode(200)
	sp.SetStatusMessage("")

	ts := &TestSpan{ro, 0}
	spans.FinishSpan(ts)
	u.Is(1, ts.finishes, "FinishSpan() calls Finish()")

	u.Is(-1, spans.NonHexIndex("0123456789abcdefABCDEF"), "valid hex")
	u.Is(16, spans.NonHexIndex("0123456789abcdefghij"), "invalid hex")
	for _, c := range ` !"#$%&'()*+,-./:;<=>?@`+
		"GHIJKLMNOPQRSTUVWXYZ[\\]^_`ghijklmnopqrstuvwxyz{|}~" {
		u.Is(0, spans.NonHexIndex(string(c)), string(c)+" not hex")
	}

	ctx := context.Background()
	u.Is(nil, spans.ContextGetSpan(ctx), "empty ContextGetSpan")
	ctx = spans.ContextStoreSpan(ctx, sp)
	u.Is(sp, spans.ContextGetSpan(ctx), "ContextGetSpan")

	u.Is(true, spans.IsValidTraceID("0123456789abcdefABCDEF1234567890"),
		"valid TraceID")
	u.Is(false, spans.IsValidTraceID("0123456789abcdegABCDEF1234567890"),
		"invalid TraceID")
	u.Is(false, spans.IsValidTraceID("0123456789abcdefABCDEF123456789"),
		"short TraceID")
	u.Is(false, spans.IsValidTraceID("0123456789abcdefABCDEF12345678901"),
		"long TraceID")
	u.Is(false, spans.IsValidTraceID("00000000000000000000000000000000"),
		"zero TraceID")
}
