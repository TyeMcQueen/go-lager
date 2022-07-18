package lager

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/TyeMcQueen/go-tutl"
)

func TestEscape(t *testing.T) {
	u := tutl.New(t)

	b := bufPool.Get().(*buffer)
	b.g = getGlobals()
	out := &bytes.Buffer{}
	b.w = out

	b.escape1Rune('"')
	u.Is(`\"`, b.buf, `esc1 "`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\\')
	u.Is(`\\`, b.buf, `esc1 \`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\b')
	u.Is(`\b`, b.buf, `esc1 \b`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\f')
	u.Is(`\f`, b.buf, `esc1 \f`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\n')
	u.Is(`\n`, b.buf, `esc1 \n`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\r')
	u.Is(`\r`, b.buf, `esc1 \r`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\t')
	u.Is(`\t`, b.buf, `esc1 \t`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\x00')
	u.Is("\\u0000", b.buf, `esc1 \x00`)
	b.buf = b.buf[0:0]
	b.escape1Rune('\x01')
	u.Is("\\u0001", b.buf, `esc1 \x01`)
	b.buf = b.buf[0:0]
	b.escape1Rune(0xF234)
	u.Is("\\uF234", b.buf, `esc1 0xF234`)
	b.buf = b.buf[0:0]

	b.escape("\x01")
	u.Is(`\u0001`, b.buf, `s:\x01`)
	b.buf = b.buf[0:0]
	b.escapeBytes([]byte{'\x01'})
	u.Is(`\u0001`, b.buf, `b:\x01`)
	b.buf = b.buf[0:0]

	b.escape("\u009e")
	u.Is(`\u009E`, b.buf, `s:\u009e`)
	b.buf = b.buf[0:0]
	b.escapeBytes([]byte("\u009e"))
	u.Is(`\u009E`, b.buf, `b:\u009e`)
	b.buf = b.buf[0:0]

	chess := "\U0001FA52\U0001FA01"
	b.escape(chess)
	u.Is(`\uD83E\uDE52\uD83E\uDE01`, b.buf, "s:chess")
	b.buf = b.buf[0:0]
	b.escapeBytes([]byte(chess))
	u.Is(`\uD83E\uDE52\uD83E\uDE01`, b.buf, "b:chess")
	b.buf = b.buf[0:0]

	b.escape("\x01 \x9A")
	u.Is(`\u0001 «x9A»`, b.buf, `s:\x01 \x9A`)
	b.buf = b.buf[0:0]
	b.escapeBytes([]byte("\x01 \x9A"))
	u.Is(`\u0001 «x9A»`, b.buf, `b:\x01 \x9A`)
	b.buf = b.buf[0:0]

	b.escape("\x01 \"\x9A\xBC\" «»")
	u.Is(`\u0001 \"«x9ABC»\" «»`, b.buf, `s:\x01 "\x9A\xBC" «»`)
	b.buf = b.buf[0:0]
	b.escapeBytes([]byte("\x01 \"\x9A\xBC\" «»"))
	u.Is(`\u0001 \"«x9ABC»\" «»`, b.buf, `b:\x01 "\x9A\xBC" «»`)
	b.buf = b.buf[0:0]

	b.int(10, 4)
	u.Is("0010", b.buf, "int(10,4)")
	b.buf = b.buf[0:0]

	b.scalar(nLevels)
	u.Is(`"11"`, b.buf, "nLevels goes to 11")
	b.buf = b.buf[0:0]

	b.w = io.Discard
	b.buf = b.buf[0 : 16*1024-10]
	b.scalar(1.0 / 3.0)
	u.Like(b.buf, "b.scalar() lock works", "^0[.]3+$")
	b.unlock()

	u.Like(
		u.GetPanic(func() {
			ContextPairs(context.WithValue(context.Background(), noop{}, 7))
		}),
		"ContextPairs imposible",
		"*invalid type", "*int not *lager.KVPairs", "*in context")
}

func TestInit(t *testing.T) {
	u := tutl.New(t)
	log := bytes.NewBuffer(nil)
	defer SetOutput(log)()

	defer Keys("", "", "", "", "", "")
	defer os.Unsetenv("LAGER_LEVELS")
	defer os.Unsetenv("LAGER_KEYS")
	defer os.Unsetenv("LAGER_GCP")
	os.Setenv("LAGER_LEVELS", "Fail Wait Note Acc Trace Obj")
	os.Setenv("LAGER_KEYS", "time,sev,msg,data,,mod")
	os.Setenv("LAGER_GCP", "1")
	firstInit()
	g := getGlobals()
	u.Is("FWNATO", g.enabled, "enabled levels")
	u.Is("time", g.keys.when, "when key")
	u.Is("sev", g.keys.lev, "lev key")
	u.Is("msg", g.keys.msg, "msg key")
	u.Is("data", g.keys.args, "args key")
	u.Is("", g.keys.ctx, "ctx key")
	u.Is("mod", g.keys.mod, "mod key")
	u.Is(true, g.inGcp, "inGcp")

	u.Is(nil, u.GetPanic(func() {
		defer ExitViaPanic()(func(x *int) { *x = -1 })
		os.Setenv("LAGER_KEYS", "time,,msg,data,,mod")
		firstInit()
	}), "init no panic")
	u.Like(log.Bytes(), "bad LAGER_KEYS",
		"*Only keys for msg and ctx can be blank")

	u.Is(nil, u.GetPanic(func() {
		defer ExitViaPanic()(func(x *int) { *x = -1 })
		Keys("time", "sev", "msg", "data", "", "")
		firstInit()
	}), "init no panic")
	u.Like(log.Bytes(), "bad LAGER_KEYS",
		"*Only keys for msg and ctx can be blank")

	u.Is(nil, u.GetPanic(func() {
		defer ExitViaPanic()(func(x *int) { *x = -1 })
		os.Setenv("LAGER_KEYS", "time,lev")
		firstInit()
	}), "init no panic")
	u.Like(log.Bytes(), "bad LAGER_KEYS",
		"*LAGER_KEYS expected 6 comma-separated labels")

	u.Is("not lager", u.GetPanic(func() {
		defer ExitViaPanic()()
		panic("not lager")
	}), "non-Exit panic")

	defer updateGlobals(setRunningInGcp(false))
}
