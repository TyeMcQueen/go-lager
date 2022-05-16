package lager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/TyeMcQueen/go-lager"
	"github.com/TyeMcQueen/go-tutl"
)

var _ = io.EOF
var _ = os.Stdout

var fakeMessage = "Test logging, but use a somewhat realistic message length."

func TestLager(t *testing.T) {
	u := tutl.New(t)
	ctx := context.Background()
	ctx = lager.AddPairs(ctx, "ip", "10.0.1.2")
	ctx = lager.AddPairs(ctx, "user", "tye")
	ctx = lager.AddPairs(ctx, "ip", "10.1.2.3")
	log := bytes.NewBuffer(nil)
	lager.OutputDest = log

	lager.Info().List("Not output")
	u.Is(0, log.Len(), "Info not logged")
	log.Reset()

	lager.Fail(ctx).List("This\x01", 1.1, "is", "out\tput\n")
	u.Like(log.Bytes(), `log 1 "ip" before "user"`, `"ip":.*"user":`)
	data := make([]interface{}, 0, 4)
	err := json.Unmarshal(log.Bytes(), &data)
	if !u.Is(nil, err, "log 1 is json list") {
		u.Log("log 1 was", log.Bytes())
	} else {
		u.Is(4, len(data), "log 1 len")
		if u.HasType("string", data[0], "log 1.0 type") {
			_, err := time.Parse("2006-01-02 15:04:05.9Z", data[0].(string))
			u.Is(nil, err, "log 1.0 is valid timestamp")
		}
		u.HasType("string", data[1], "log 1.1 type")
		u.Is("FAIL", data[1], "log 1.1")
		if u.HasType("[]interface {}", data[2], "log 1.2 type") {
			l := data[2].([]interface{})
			u.Is(4, len(l), "log 1.2 len")
			u.Is("This\x01", l[0], "log 1.2.0")
			u.HasType("float64", l[1], "log 1.2.1 type")
			u.Is(1.1, l[1], "log 1.2.1")
			u.Is("is", l[2], "log 1.2.2")
			u.Is("out\tput\n", l[3], "log 1.2.3")
		}
		if u.HasType("map[string]interface {}", data[3], "log 1.3 type") {
			h := data[3].(map[string]interface{})
			u.Is(2, len(h), "log 1.3 len")
			u.Is("10.1.2.3", h["ip"], "log 1.3.ip")
			u.Is("tye", h["user"], "log 1.3.user")
		}
	}
	log.Reset()

	lager.Warn().Map("Output?", true, 1.45)
	err = json.Unmarshal(log.Bytes(), &data)
	if !u.Is(nil, err, "log 2 is json list") {
		u.Log("log 2 was", log.Bytes())
	} else {
		u.Is(3, len(data), "log 2 len")
		u.Like(data[0], "log 2.0",
			"^[0-9]{4}-[0-1][0-9]-[0-3][0-9] ",
			" [012][0-9]:[0-5][0-9]:[0-5][0-9][.][0-9]+Z$")
		u.Is("WARN", data[1], "log 2.1")
		if u.HasType("map[string]interface {}", data[2], "log 2.2 type") {
			h := data[2].(map[string]interface{})
			u.Is(2, len(h), "log 2.2 len")
			u.HasType("bool", h["Output?"], "log 2.2.output type")
			u.Is(true, h["Output?"], "log 2.2.output")
			ix, ok := h["1.45"]
			u.Is("<nil>", ix, "log 2.2[1.45]")
			u.Is(true, ok, "log 2.2[1.45] exists")
		}
	}
	log.Reset()

	// TODO...
	mod := lager.NewModule(`mod"test"`)
	mod.Fail(ctx).List("From a module")
}

func BenchmarkLog(b *testing.B) {
	lager.OutputDest = ioutil.Discard
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
