package lager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TyeMcQueen/go-lager"
	"github.com/TyeMcQueen/go-tutl"
)

var _ = os.Stdout

func validJson(what string, b []byte, pDest interface{}, u tutl.TUTL) bool {
	u.Helper()
	var whatev interface{}
	if nil == pDest {
		pDest = &whatev
	}
	err := json.Unmarshal(b, pDest)
	if !u.Is(nil, err, what+" is valid JSON") {
		u.Log(what+" was", string(b))
		return false
	}
	return true
}

func TestLager(t *testing.T) {
	u := tutl.New(t)
	ctx := context.Background()
	ctx2 := lager.AddPairs(ctx, "ip", "10.1.2.3")
	ctx = lager.AddPairs(ctx, "ip", "10.0.1.2")
	ctx = lager.AddPairs(ctx, "user", lager.S("tye"))
	u.Is(lager.ContextPairs(ctx), lager.ContextPairs(lager.AddPairs(ctx)),
		"lager.AddPairs(ctx) is no-op")
	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()

	ll := lager.Info()
	u.Is(false, ll.Enabled(), "not enabled")
	ll.List("Not output")
	u.Is(0, log.Len(), "Info not logged")
	log.Reset()

	ll = lager.Note(ctx, ctx2)
	u.Is(true, ll.Enabled(), "is enabled")
	ll.List("This\x01", 1.1, "is", "out\tput\n")
	u.Like(log.Bytes(), `log 1 "ip" before "user"`, `"ip":.*"user":`)
	list := make([]interface{}, 0, 4)
	if validJson("log 1", log.Bytes(), &list, u) {
		u.Is(4, len(list), "log 1 len")
		if u.HasType("string", list[0], "log 1.0 type") {
			_, err := time.Parse("2006-01-02 15:04:05.9Z", list[0].(string))
			u.Is(nil, err, "log 1.0 is valid timestamp")
		}
		u.HasType("string", list[1], "log 1.1 type")
		u.Is("NOTE", list[1], "log 1.1")
		if u.HasType("[]interface {}", list[2], "log 1.2 type") {
			l := list[2].([]interface{})
			u.Is(4, len(l), "log 1.2 len")
			u.Is("This\x01", l[0], "log 1.2.0")
			u.HasType("float64", l[1], "log 1.2.1 type")
			u.Is(1.1, l[1], "log 1.2.1")
			u.Is("is", l[2], "log 1.2.2")
			u.Is("out\tput\n", l[3], "log 1.2.3")
		}
		if u.HasType("map[string]interface {}", list[3], "log 1.3 type") {
			h := list[3].(map[string]interface{})
			u.Is(2, len(h), "log 1.3 len")
			u.Is("10.1.2.3", h["ip"], "log 1.3.ip")
			u.Is("tye", h["user"], "log 1.3.user")
		}
	}
	log.Reset()

	lager.Warn().WithCaller(99).Map("Output?", true, 1.45)
	if validJson("log 2", log.Bytes(), &list, u) {
		u.Is(3, len(list), "log 2 len")
		u.Like(list[0], "log 2.0",
			"^[0-9]{4}-[0-1][0-9]-[0-3][0-9] ",
			" [012][0-9]:[0-5][0-9]:[0-5][0-9][.][0-9]+Z$")
		u.Is("WARN", list[1], "log 2.1")
		if u.HasType("map[string]interface {}", list[2], "log 2.2 type") {
			h := list[2].(map[string]interface{})
			u.Is(2, len(h), "log 2.2 len")
			u.HasType("bool", h["Output?"], "log 2.2.output type")
			u.Is(true, h["Output?"], "log 2.2.output")
			ix, ok := h["1.45"]
			u.Is("<nil>", ix, "log 2.2[1.45]")
			u.Is(true, ok, "log 2.2[1.45] exists")
		}
	}
	log.Reset()

	lager.Keys("t", "l", "m", "data", "", "mod")

	lager.SetPathParts(3)
	lager.Fail(ctx).WithStack(0, 1).MMap("message", "key", "value")
	hash := make(map[string]interface{})
	if validJson("log 3", log.Bytes(), &hash, u) {
		u.Is(7, len(hash), "log 3 len")
		u.Like(log.Bytes(), "log 3 key order",
			`"t":.*"l":.*"m":.*"key":.*"ip":.*"user":`)
		u.Like(hash["t"], "log 3.t",
			"^[0-9]{4}-[0-1][0-9]-[0-3][0-9]T",
			"T[012][0-9]:[0-5][0-9]:[0-5][0-9][.][0-9]+Z$")
		u.Is("FAIL", hash["l"], "log 3.l")
		u.Is("message", hash["m"], "log 3.m")
		u.Is("value", hash["key"], "log 3.key")
		u.Is("10.0.1.2", hash["ip"], "log 3.ip")
		u.Is("tye", hash["user"], "log 3.user")
		u.Like(hash["_stack"], "log 3._stack",
			`^\[[1-9][0-9]* [^/ ]+/[^/ ]+/lager_test[.]go\]$`)
	}
	log.Reset()

	lager.Keys("", "", "", "", "", "")

	// TODO
	mod := lager.NewModule(`mod"test"`)
	mod.Fail(ctx).List("From a module")
	u.Is(true, lager.SetModuleLevels(`mod"test"`, "FW"), "set mod lev")
	if validJson("log 4", log.Bytes(), &list, u) {
		u.Is(5, len(list), "log 4 len")
	/*  u.Like(list[0], "log 4.0",
			"^[0-9]{4}-[0-1][0-9]-[0-3][0-9] ",
			" [012][0-9]:[0-5][0-9]:[0-5][0-9][.][0-9]+Z$")
		u.Is("WARN", list[1], "log 4.1")
		h, ok := list[2].(map[string]interface{})
		if !u.Is(true, ok, "log 4.2 is hash") {
			u.Log("log 4.2 type", fmt.Sprintf("%T", list[2]))
		} else {
			u.Is(2, len(h), "log 4.2 len")
			_, ok = h["Output?"].(bool)
			u.Is("true", h["Output?"], "log 4.2")
			u.Is(true, ok, "log 4.2 is bool")
			ix, ok := h["1.45"]
			u.Is("<nil>", ix, "log 4.2.'1.45'")
			u.Is(true, ok, "log 4.2.'1.45' exists")
		}
		h, ok = list[3].(map[string]interface{})
		if !u.Is(true, ok, "log 4.3 is list") {
			u.Log("log 4.2 type", fmt.Sprintf("%T", list[3]))
			u.Log("log 4.2 value", list[3])
		} else {
			u.Is(2, len(h), "log 4.3 len")
			u.Is("10.1.2.3", h["ip"], "log 4.3.ip")
			u.Is("tye", h["user"], "log 4.3.user")
		} */
	}
	log.Reset()
}

func TestData(t *testing.T) {
	u := tutl.New(t)
	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()
	lager.Init("FAWN BIT A DOG")
	defer lager.Init("FWNA")

	lager.Keys("t", "l", "", "data", "", "mod")

	lager.SetPathParts(1)
	chess := "\U0001FA52\U0001FA01"
	repl := "\uFFFD"
	lager.Acc().CMMap(
		"( \\ \b \f \r \000 \x7F\u0081 "+repl+"\x80\xBF \uFB01 "+chess+" )",
		"ok",
		func() interface{} { return "okay" },
		"odd",
		struct {
			S string
			F func() interface{}
		}{
			"no oops",
			func() interface{} { return "ooops" },
		},
		"json",
		struct {
			I int
			S string
		}{1, "str"},
		lager.InlinePairs,
		lager.Pairs("pair", "value"),
		lager.InlinePairs,
		lager.Map("map", "second"),
		lager.InlinePairs,
		lager.List("item"),
		lager.InlinePairs,
		*lager.Pairs("kv", "pairs"),
	)
	hash := make(map[string]interface{})
	if validJson("log d1", log.Bytes(), &hash, u) {
		u.Is(12, len(hash), "log d1 len")
		u.Is("( \\ \b \f \r \000 \x7F\u0081 "+repl+"«x80BF» \uFB01 "+chess+" )",
			hash["msg"], "log d1.m")
		u.Like(log.Bytes(), "log d1",
			`"[(] \\\\ \\b \\f \\r \\u0000 \\u007F\\u0081 `+
				repl+"«x80BF» \uFB01 "+`(\\u[0-9A-F]{4}){4} [)]`)
		u.Is("ACCESS", hash["l"], "log d1.l")
		u.Is("okay", hash["ok"], "log d1.ok")
		u.Like(hash["odd"], "log.d1.odd",
			"*! json: unsupported type: func() interface {};",
			`*struct { S string; F func() interface {} }{S:"no oops",`+
				` F:(func() interface {})(0x`, //)
			` F:[(]func[(][)] interface {}[)][(]0x[0-9a-fA-F]+[)]}`,
			"!ooops",
		)
		u.HasType("map[string]interface {}", hash["json"], "log.d1.json")
		u.Is("map[I:1 S:str]", hash["json"], "log.d1.json")
		u.Is("lager_test.go", hash["_file"], "log.d1._file")
		u.Like(hash["_line"], "log.d1._line", "^[1-9][0-9]*$")
		u.Is("value", hash["pair"], "log.d1.pair")
		u.Is("second", hash["map"], "log.d1.map")
		u.Is("pairs", hash["kv"], "log.d1.kv")
		u.Is("[item]", hash["cannot-inline"], "log.d1.cannot-inline")
		u.HasType("[]interface {}", hash["cannot-inline"],
			"log.d1.cannot-inline type")
	}
	log.Reset()

	lager.Init("FailWarnNoteAccInfoTraceDebugObjGuts")

	ran := false
	lager.Info().CMap(
		lager.Unless(true, "not used"),
		func() interface{} {
			ran = true
			return "oops"
		},
		"ugh",
		strings.Repeat("ohno!", 4*1024),
		"slow",
		func() interface{} {
			time.Sleep(11*time.Millisecond)
			return "okay"
		},
		lager.Unless(false, "fast"),
		func() interface{} { return "okay" },
	)
	u.Is(false, ran, "func ran despite Unless")
	hash = make(map[string]interface{})
	if validJson("log d2", log.Bytes(), &hash, u) {
		u.Is(7, len(hash), "log d2 len")
		u.Is(nil, hash["not used"], "log d2[not used]")
		u.Is("INFO", hash["l"], "log d2.l")
		u.HasType("string", hash["ugh"], "log d2.ugh type")
		u.Is("okay", hash["fast"], "log d2.fast")
		u.Like(hash["slow"], "log.d2.slow",
			"*func call took more than 10ms while lager lock held",
			"*(log line was already over 16KiB)",
		)
	}
	log.Reset()

	uri, _ := url.Parse("http://localhost/")
	lager.Trace().CList(
		false,
		math.Inf(1), float32(math.Inf(-1)),
		int8(-8), int16(-16), int32(-32), int64(-64),
		uint8(8), uint16(16), uint32(32), uint64(64), uint(1),
		float32(1.32),
		[]byte("[]byte"),
		[]string{"[]","string"},
		map[string]interface {}{
			"string": "interface{}",
		},
		io.EOF,
		uri, // A String()er
	)
	hash = make(map[string]interface{})
	if validJson("log d3", log.Bytes(), &hash, u) {
		u.Is(5, len(hash), "log d3 len")
		u.Is("TRACE", hash["l"], "log d3.l")
		if u.HasType("[]interface {}", hash["data"], "log d3.data type") {
			list := hash["data"].([]interface{})
			u.Is(false, list[0], "log d3.data.0")
			u.HasType("bool", list[0], "log d3.data.0 type")
			u.Is("+Inf", list[1], "log d3.data.1")
			u.HasType("string", list[1], "log d3.data.1 type")
			u.Is("-Inf", list[2], "log d3.data.2")
			u.HasType("string", list[2], "log d3.data.2 type")
			u.Is(-8, list[3], "log d3.data.3")
			u.Is(-16, list[4], "log d3.data.4")
			u.Is(-32, list[5], "log d3.data.5")
			u.Is(-64, list[6], "log d3.data.6")
			u.Is(8, list[7], "log d3.data.7")
			u.Is(16, list[8], "log d3.data.8")
			u.Is(32, list[9], "log d3.data.9")
			u.Is(64, list[10], "log d3.data.10")
			u.Is(1, list[11], "log d3.data.11")
			u.Is(1.32, list[12], "log d3.data.12")
			u.Is("[]byte", list[13], "log d3.data.13")
			u.HasType("string", list[13], "log d3.data.13 type")
			u.HasType("[]interface {}", list[14], "log d3.data.14 type")
			u.Is("[[] string]", list[14], "log d3.data.14")
			u.HasType("map[string]interface {}", list[15], "log d3.data.15 type")
			u.Is("map[string:interface{}]", list[15], "log d3.data.15")
			u.Is("EOF", list[16], "log d3.data.16")
			u.Is("http://localhost/", list[17], "log d3.data.17")
		}
	}
	log.Reset()

	dones := make(chan bool, 1)
	guts := bytes.Repeat([]byte("<.>"), 6*1024)
	lager.Guts().CMList(
		"message",
		"guts",
		guts,
		"can't",
		func() interface{} {
			lager.Obj().List("deadlock")
			dones <- true
			return "ooops"
		},
	)
	<-dones
	lines := bytes.Split(log.Bytes(), []byte{'\n'})
	if u.Is(3, len(lines), "lines log from deadlock") {
		u.Is(0, len(lines[2]), "last line from deadlock len")
		validJson("deadlock 1", lines[0], nil, u)
		validJson("deadlock 2", lines[1], nil, u)
	}
	u.Like(log.Bytes(), "deadlock",
		`^{.*"func call took.*}\n{.*"deadlock"`)
	log.Reset()

	b := []byte("bytes")
	s := lager.S(b)
	u.Is(s, b, "lager.S([]byte) faithful")
	u.HasType("string", s, "lager.S([]byte) type")

	pair := lager.Pairs("one", "pair")
	u.Is(true, pair == pair.AddPairs(), "add no pairs to AMap is no-op")
	u.Is(true, pair == pair.Merge(&lager.KVPairs{}), "merge edge case")
	u.Is("&{[one] [two]}", pair.AddPairs("one", "two"), "pair key conflict")

	lager.Init("FWNA")
}

func TestFormat(t *testing.T) {
	u := tutl.New(t)
	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()

	lager.Keys("", "", "", "", "", "")
	lager.Warn().List()
	validJson("List() no args w/o key", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "List() no args w/o key", `"WARN", \[\]`)
	log.Reset()
	lager.Warn().Map()
	validJson("Map() no args w/o key", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "Map() no args w/o key", `"WARN", {}`)
	log.Reset()

	lager.Keys("t", "l", "msg", "data", "ctx", "mod")
	lager.Warn().List()
	validJson("List() no args w/ key", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "List() no args w/ key", `"data":\[\]`)
	log.Reset()
	lager.Warn().Map()
	validJson("Map() no args w/ key", log.Bytes(), nil, u) // {
	u.Like(log.Bytes(), "Map() no args w/ key", `"WARN"}`)
	log.Reset()
	lager.Warn().Println("one")
	validJson("Println()", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "Println()", `"msg":"one"`)
	log.Reset()

	// .List("lone")
	lone := []string{
		`"lone"]`,
		`"lone", {"extra":"value"}]`,
		`"data":["lone"]}`,
		`"data":["lone"], "ctx":{"extra":"value"}}`,
		`"m":"lone", "extra":"value"}`,
		`"m":"lone"}`,
		`"m":"lone", "ctx":{"extra":"value"}}`,
	}

	// .List("one", "two")
	two := []string{
		`["one", "two"]]`,
		`["one", "two"], {"extra":"value"}]`,
		`"data":["one", "two"]}`,
		`"data":["one", "two"], "ctx":{"extra":"value"}}`,
		`"data":["one", "two"], "extra":"value"}`,
		`"data":["one", "two"]}`,
		`"data":["one", "two"], "ctx":{"extra":"value"}}`,
	}

	// .MList("mlist")
	mlist := []string{
		`"mlist"]`,
		`"mlist", {"extra":"value"}]`,
		`"data":["mlist"]}`,
		`"data":["mlist"], "ctx":{"extra":"value"}}`,
		`"m":"mlist", "extra":"value"}`,
		`"m":"mlist"}`,
		`"m":"mlist", "ctx":{"extra":"value"}}`,
	}

	// .MList("m2list", "a", "b")
	m2list := []string{
		`["m2list", "a", "b"]]`,
		`["m2list", "a", "b"], {"extra":"value"}]`,
		`"data":["m2list", "a", "b"]}`,
		`"data":["m2list", "a", "b"], "ctx":{"extra":"value"}}`,
		`"m":"m2list", "data":["a", "b"], "extra":"value"}`,
		`"m":"m2list", "data":["a", "b"]}`,
		`"m":"m2list", "data":["a", "b"], "ctx":{"extra":"value"}}`,
	}

	// .Map("map", 1, "pam", 2)
	pam := []string{
		`{"map":1, "pam":2}]`,
		`{"map":1, "pam":2}, {"extra":"value"}]`,
		`"map":1, "pam":2}`,
		`"map":1, "pam":2, "ctx":{"extra":"value"}}`,
		`"map":1, "pam":2, "extra":"value"}`,
		`"map":1, "pam":2}`,
		`"map":1, "pam":2, "ctx":{"extra":"value"}}`,
	}

	// .MMap("mmap")
	mmap := []string{
		`"mmap"]`,
		`"mmap", {"extra":"value"}]`,
		`"msg":"mmap"}`,
		`"msg":"mmap", "ctx":{"extra":"value"}}`,
		`"m":"mmap", "extra":"value"}`,
		`"m":"mmap"}`,
		`"m":"mmap", "ctx":{"extra":"value"}}`,
	}

	// .MMap("m2map", "map", 1, "pam", 2)
	m2map := []string{
		`"m2map", {"a":1, "b":2}]`,
		`"m2map", {"a":1, "b":2}, {"extra":"value"}]`,
		`"msg":"m2map", "a":1, "b":2}`,
		`"msg":"m2map", "a":1, "b":2, "ctx":{"extra":"value"}}`,
		`"m":"m2map", "a":1, "b":2, "extra":"value"}`,
		`"m":"m2map", "a":1, "b":2}`,
		`"m":"m2map", "a":1, "b":2, "ctx":{"extra":"value"}}`,
	}

	o := 0
	check := func(want string) {
		u.Helper()
		b := log.Bytes()
		validJson(u.S(o), log.Bytes(), nil, u)
		b[len(b)-1] = '.'
		u.Like(b, u.S(o), "*"+want+".")
		log.Reset()
	}

	plain := context.Background()
	extra := lager.AddPairs(plain, "extra", "value")
	for _, msg := range []string{"", "m"} {
		for _, ctx := range []string{"", "ctx"} {
			lager.Keys("t", "l", msg, "data", ctx, "mod")
			for _, pairs := range []context.Context{plain, extra} {
				if "" == ctx && pairs == plain {
					if "" != msg {
						continue
					}
					lager.Keys("", "", "", "", "", "")
				}
				ll := lager.Note(pairs)
				ll.List("lone")
				check(lone[o])
				ll.List("one", "two")
				check(two[o])
				ll.MList("mlist")
				check(mlist[o])
				ll.MList("m2list", "a", "b")
				check(m2list[o])
				ll.Map("map", 1, "pam", 2)
				check(pam[o])
				ll.MMap("mmap")
				check(mmap[o])
				ll.MMap("m2map", "a", 1, "b", 2)
				check(m2map[o])
				o++
			}
		}
	}

	lager.RunningInGcp()
	defer lager.SetLevelNotation(nil)
	ll := lager.Note()
	ll.List("str")
	validJson("gcp lone", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "gcp lone",
		`*"message":"str", "json":1}`)
	log.Reset()
	ll.MList("str")
	validJson("gcp mlist", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "gcp mlist",
		`*"message":"str", "json":1}`)
	log.Reset()
	ll.MMap("str")
	validJson("gcp mmap", log.Bytes(), nil, u)
	u.Like(log.Bytes(), "gcp mmap",
		`*"message":"str", "json":1}`)
	log.Reset()

	// Get 100% coverage until next release:
	ll.WithCaller(1, 1).WithStack(1, 1, 1)
}

func TestExit(t *testing.T) {
	u := tutl.New(t)
	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()

	lager.ExitNotExpected(true)
	defer lager.ExitNotExpected(false)
	u.Like(log.Bytes(), "warn exit",
		"*ExitNotExpected(true) when ExitViaPanic() not enabled")
	log.Reset()

	defer func() {
		u.Like(log.Bytes(), "log exit", `"Exiting"`,
			`"EXIT"`, `"_stack":\["[1-9][0-9]* lager_test.go", "`)
		log.Reset()
	}()

	defer lager.ExitViaPanic()(func(x *int) { *x = -1 })

	lager.Exit().List("Exiting")
}

func TestLevels(t *testing.T) {
	u := tutl.New(t)

	lager.Init("FWNAITDOG")

	byLetter, byMethod := lager.Level('P'), lager.Panic()
	u.Is(true, byLetter == byMethod, "Panic")
	byLetter, byMethod = lager.Level('E'), lager.Exit()
	u.Is(true, byLetter == byMethod, "Exit")
	byLetter, byMethod = lager.Level('F'), lager.Fail()
	u.Is(true, byLetter == byMethod, "Fail")
	byLetter, byMethod = lager.Level('W'), lager.Warn()
	u.Is(true, byLetter == byMethod, "Warn")
	byLetter, byMethod = lager.Level('N'), lager.Note()
	u.Is(true, byLetter == byMethod, "Note")
	byLetter, byMethod = lager.Level('I'), lager.Info()
	u.Is(true, byLetter == byMethod, "Info")
	byLetter, byMethod = lager.Level('A'), lager.Acc()
	u.Is(true, byLetter == byMethod, "Acc")
	byLetter, byMethod = lager.Level('T'), lager.Trace()
	u.Is(true, byLetter == byMethod, "Trace")
	byLetter, byMethod = lager.Level('D'), lager.Debug()
	u.Is(true, byLetter == byMethod, "Debug")
	byLetter, byMethod = lager.Level('O'), lager.Obj()
	u.Is(true, byLetter == byMethod, "Obj")
	byLetter, byMethod = lager.Level('G'), lager.Guts()
	u.Is(true, byLetter == byMethod, "Guts")

	lager.Init("FAWN")

	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()

	ll := lager.Debug().With().WithCaller(1).WithStack(0, 1)
	u.Is(false, ll.Enabled(), "disabled level")
	ll.List()
	ll.Map()
	ll.MList("no-op")
	ll.MMap("no-op")
	ll.CList()
	ll.CMap()
	ll.CMList("no-op")
	ll.CMMap("no-op")
	ll.Println("no-op")
	u.Is("", log.Bytes(), "disabled never logs")

	u.Like(u.GetPanic(func() { lager.Level('Q') }), "Level(Q)",
		"*must be", `"PEFWNAITDOG"`, "not 'Q'")
}

func TestPanic(t *testing.T) {
	u := tutl.New(t)
	log := bytes.NewBuffer(nil)
	defer lager.SetOutput(log)()

	u.Like(u.GetPanic(func() { lager.Panic().List("panic test") }),
		"panic panic", "lager.Panic[(][)] logged", "*see above")
	u.Like(log.Bytes(), "panic logged", `"panic test"`, `"PANIC"`)
}

var fakeMessage = "Test logging, but use a somewhat realistic message length."

func BenchmarkLog(b *testing.B) {
	defer lager.SetOutput(io.Discard)()
	lager.Fail().List("Initialize things")
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lager.Fail().List()
			lager.Fail().Map("msg", fakeMessage, "size", 45)
			lager.Fail().List("Is message short and simple?", true)
			lager.Fail().Map("Failure", io.EOF, "Pos", 12345, "Percent", 12.345)
		}
	})
}
