package lager

import(
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
)


// TYPES //

// Just an alias for context.Context that takes up less space in function
// signatures.  You never need to use lager.Ctx in your code.
type Ctx = context.Context

type Lager interface {
	// Writes a single log line to Stdout in JSON format encoding a UTC time-
	// stamp followed by the log level and the passed-in values.  For example:
	//   lager.Warn().List("Timeout", dest, secs)
	// might output:
	//   ["2018-12-31 23:59:59.086Z", "WARN", ["Timeout", "dbhost", 5]]
	// Or logs nothing if the corresponding log level is not enabled.
	List(args ...interface{})

	// Writes a single log line to Stdout in JSON format encoding a list of
	// a UTC timestamp (string), the log level (string), and a map composed
	// of the key/value pairs passed in.  For example:
	//   lager.Warn().Map("Err", err, "for", obj)
	// might output:
	//   ["2018-12-31 23:59:59.086Z", "WARN", {"Err":"no cost", "for":{"c":-1}}]
	// Or logs nothing if the corresponding log level is not enabled.
	Map(pairs ...interface{})

	// Gets a new Lager that adds to each log line the key/value pairs from
	// zero or more context.Context values.
	With(ctxs ...context.Context) Lager

	// Does this Lager log anything?
	Enabled() bool
}

// A stub Lager that outputs nothing:
type noop struct{}  // Also used as "key" for context.Context decoration.
func (_ noop) List(_ ...interface{}) {}
func (_ noop) Map(_ ...interface{}) {}
func (n noop) With(_ ...Ctx) Lager { return n }
func (_ noop) Enabled() bool { return false }

type level int8
const(
	lPanic level = iota
	lExit; lFail; lWarn; lAcc; lInfo; lTrace; lDebug; lObj; lGuts
	nLevels
)

// A Lager that actually logs.
type logger struct {
	lev level       // Log level
	kvp *KVPairs    // Extra key/value pairs to append to each log line.
	mod string      // The module name where the log level is en/disabled.
}


// GLOBALS //

// A Lager singleton for each log level (or a noop).
var _lagers [int(nLevels)]Lager

// The currently enabled log levels (used by module.go).
var _enabledLevels string

// Set to a non-nil io.Writer to not write logs to os.Stdout and os.Stderr.
var OutputDest io.Writer


// FUNCS //

func init() {
	_lagers[int(lPanic)] = &logger{lev: lPanic}
	_lagers[int(lExit)] = &logger{lev: lExit}
	Init(os.Getenv("LAGER_LEVELS"))
}

// En-/disables log levels.  Pass in a string of letters from "FWAITDOG" to
// indicate which log levels should be the only ones that produce output.
// Each letter is the first letter of a log level (Fail, Warn, Acc, Info,
// Trace, Debug, Obj, or Guts).   Levels Panic and Exit are always enabled.
// Init("") acts like Init("FWA"), the default setting.  To disable all
// optional logs, you can use Init("-") as any characters not from "FWAITDOG"
// are silently ignored.  So you can also call Init("Fail Warn Acc Info").
func Init(levels string) {
	_enabledLevels = ""
	for l := lFail; l <= lGuts; l++ {
		_lagers[int(l)] = noop{}
	}
	if "" == levels {
		levels = "FW"
	}
	for _, c := range levels {
		switch c {
		case 'F': _lagers[int(lFail)]  = &logger{lev: lFail}
		case 'W': _lagers[int(lWarn)]  = &logger{lev: lWarn}
		case 'A': _lagers[int(lAcc)]   = &logger{lev: lAcc}
		case 'I': _lagers[int(lInfo)]  = &logger{lev: lInfo}
		case 'T': _lagers[int(lTrace)] = &logger{lev: lTrace}
		case 'D': _lagers[int(lDebug)] = &logger{lev: lDebug}
		case 'O': _lagers[int(lObj)]   = &logger{lev: lObj}
		case 'G': _lagers[int(lGuts)]  = &logger{lev: lGuts}
		default:  continue
		}
		_enabledLevels += strconv.QuoteRune(c)
	}
}

func forLevel(lev level, cs ...Ctx) Lager {
	return _lagers[int(lev)].With(cs...)
}

// Returns a Lager object that calls panic().  The JSON log line is first
// output to os.Stderr and then
//    panic("lager.Panic() logged (see above)")
// is called.
func Panic(cs ...Ctx) Lager { return forLevel(lPanic, cs...) }

// Returns a Lager object that writes to os.Stderr then calls os.Exit(1).
// This log level is often called "Fatal" but loggers are inconsistent as to
// whether logging at the Fatal level causes the process to exit.  By naming
// this level Exit, that ambiguity is removed.
func Exit(cs ...Ctx) Lager { return forLevel(lExit, cs...) }

// Returns a Lager object.  If Fail log level has been disabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report errors that are not part of the normal flow.
func Fail(cs ...Ctx) Lager { return forLevel(lFail, cs...) }

// Returns a Lager object.  If Warn log level has been disabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report unusual conditions that may be signs of problems.
func Warn(cs ...Ctx) Lager { return forLevel(lWarn, cs...) }

// Returns a Lager object.  If Acc log level has been disabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to write access logs.  The level is recorded as "ACCESS".
func Acc(cs ...Ctx) Lager { return forLevel(lAcc, cs...) }

// Returns a Lager object.  If Info log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report major milestones that are part of normal flow.
func Info(cs ...Ctx) Lager { return forLevel(lInfo, cs...) }

// Returns a Lager object.  If Trace log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to trace how execution is flowing through the code.
func Trace(cs ...Ctx) Lager { return forLevel(lTrace, cs...) }

// Returns a Lager object.  If Debug log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to log important details that may help in debugging.
func Debug(cs ...Ctx) Lager { return forLevel(lDebug, cs...) }

// Returns a Lager object.  If Obj log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to log the details of internal data structures.
func Obj(cs ...Ctx) Lager { return forLevel(lObj, cs...) }

// Returns a Lager object.  If Guts log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level for debugging data that is too voluminous to always include
// when debugging.
func Guts(cs ...Ctx) Lager { return forLevel(lGuts, cs...) }

// Pass in one character from "PEFWAITDOG" to get a Lager object that either
// logs or doesn't, depending on whether the specified log level is enabled.
func Level(lev byte, cs ...Ctx) Lager {
	switch lev {
	case 'P': return forLevel(lPanic, cs...)
	case 'E': return forLevel(lExit, cs...)
	case 'F': return forLevel(lFail, cs...)
	case 'W': return forLevel(lWarn, cs...)
	case 'A': return forLevel(lAcc, cs...)
	case 'I': return forLevel(lInfo, cs...)
	case 'T': return forLevel(lTrace, cs...)
	case 'D': return forLevel(lDebug, cs...)
	case 'O': return forLevel(lObj, cs...)
	case 'G': return forLevel(lGuts, cs...)
	}
	panic(fmt.Sprintf(
		"Level() must be one char from \"PEFWAITDOG\" not %q", lev))
}

var levNames = map[level]string{
	lPanic: "PANIC",
	lExit:  "EXIT",
	lFail:  "FAIL",
	lWarn:  "WARN",
	lAcc:   "ACCESS",
	lInfo:  "INFO",
	lTrace: "TRACE",
	lDebug: "DEBUG",
	lObj:   "OBJ",
	lGuts:  "GUTS",
}

func (l level) String() string {
	name := levNames[l]
	if "" != name {
		return name
	}
	return fmt.Sprintf("%d", int(l))
}

func (l *logger) Enabled() bool { return true }

func (l *logger) With(ctxs ...Ctx) Lager {
	kvp := l.kvp
	for _, ctx := range ctxs {
		kvp = kvp.Merge(ContextPairs(ctx))
	}
	if kvp == l.kvp {
		return l
	}
	cp := *l
	cp.kvp = kvp
	return &cp
}

// Common opening steps for both List() and Map() methods.
func (l *logger) start() *buffer {
	b := bufPool.Get().(*buffer)
	switch l.lev {
	case lPanic, lExit:
		b.w = os.Stderr
	default:
		b.w = os.Stdout
	}
	if nil != OutputDest {
		b.w = OutputDest
	}
	b.open("[")
	b.timestamp()
	b.quote(l.lev.String())
	return b
}

// Common closing steps for both List() and Map() methods.
func (l *logger) end(b *buffer) {
	b.scalar(l.kvp)
	if "" != l.mod {
		b.write(b.delim, `"module=`)
		b.escape(l.mod)
		b.write(`"`)
	}
	b.close("]\n")
	b.delim = ""
	b.unlock()
	bufPool.Put(b)

	switch l.lev {
	case lExit:  os.Exit(1)
	case lPanic: panic("lager.Panic() logged (see above)")
	}
}

// Log a list of values (see the Lager interface for more details).
func (l *logger) List(args ...interface{}) {
	b := l.start()
	b.scalar(args)
	l.end(b)
}

// Log a map of key/value pairs (see the Lager interface for more details).
func (l *logger) Map(pairs ...interface{}) {
	b := l.start()
	b.scalar(RawMap(pairs))
	l.end(b)
}
