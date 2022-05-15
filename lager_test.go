package lager_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/TyeMcQueen/go-lager"
)

var _ = io.EOF
var _ = os.Stdout

var fakeMessage = "Test logging, but use a somewhat realistic message length."

func TestLager(t *testing.T) {
	ctx := context.Background()
	ctx = lager.AddPairs(ctx, "ip", "10.0.1.2")
	ctx = lager.AddPairs(ctx, "user", "tye")
	ctx = lager.AddPairs(ctx, "ip", "10.1.2.3")
	log := bytes.NewBuffer(nil)
	lager.OutputDest = log
	lager.Info().List("Not output")
	lager.Fail(ctx).List("This\x01", 1.1, "is", "out\tput\n")
	lager.Warn(ctx).Map("Output?", true, 1.45)

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
