package lager_test

import(
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"gitlab.internal.unity3d.com/sre/lager"
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func TestZero(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Info().Msg(fakeMessage)
}

func TestZapS(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	sugar.Infow(fakeMessage)
}

func TestZapL(t *testing.T) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	logger.Info(fakeMessage)
}



func BenchmarkLog(b *testing.B) {
	lager.OutputDest = ioutil.Discard
	lager.Fail().List("Initialize things")
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
//  for i := 0; i < b.N; i++ {
	//  lager.Fail().List()
		lager.Fail().Map("msg", fakeMessage, "size", 45)
	//  lager.Fail().List("Is message short and simple?", true)
	//  lager.Fail().Map("Failure", io.EOF, "Pos", 12345, "Percent", 12.345)
//  }
	}})
}


func BenchmarkZero(b *testing.B) {
	logger := zerolog.New(ioutil.Discard).With().Timestamp().Logger()
//  logger := zerolog.New(os.Stdout)
	b.ResetTimer()
	b.ReportAllocs()
//  for i := 0; i < b.N; i++ {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info().Int("size", 45).Msg(fakeMessage)
	}})
//  }
}


type Discarder struct {
	io.Writer
}
func (_ Discarder) Sync() error { return nil }

var discard = Discarder{Writer: ioutil.Discard}

func BenchmarkZapS(b *testing.B) {
	logger := zap.New( zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionConfig().EncoderConfig),
		discard,
		zap.DebugLevel,
	) )
	defer logger.Sync()
	sugar := logger.Sugar()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sugar.Infow(fakeMessage, "size", 45)
	}})
}

func BenchmarkZapL(b *testing.B) {
//  logger, _ := zap.NewProduction()
	logger := zap.New( zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionConfig().EncoderConfig),
		discard,
		zap.DebugLevel,
	) )
	defer logger.Sync()
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info(fakeMessage, zap.Int("size",45))
	}})
}
