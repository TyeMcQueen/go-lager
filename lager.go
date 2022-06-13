package lager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

/// TYPES ///

// Just an alias for context.Context that takes up less space in function
// signatures.  You never need to use lager.Ctx in your code.
type Ctx = context.Context

// 'Lager' is the interface returned from lager.Warn() and the other
// log-level selectors.
type Lager interface {
	// List() writes a single log line in JSON format including a UTC
	// timestamp, log level, and the passed-in values.  Or logs nothing if
	// the corresponding log level is not enabled.
	//
	// You may prefer MMap() or MList() if using common log processors.
	//
	List(args ...interface{})

	// CList() is the same as '.WithCaller(0,-1).List(...)'.
	CList(args ...interface{})

	// MList() takes a message string followed by 0 or more arbitrary values.
	//
	// If Keys() have not been set, then MList() acts similar to List().  If
	// Keys() have been set, then MList() acts similar to:
	//
	//      MMap(message, "data", lager.List(args...))
	//
	// except the "data" key is taken from the Keys() config.
	//
	MList(message string, args ...interface{})

	// CMList() is the same as '.WithCaller(0,-1).MList(...)'.
	CMList(message string, args ...interface{})

	// Map() takes a list of key/value pairs and writes a single log line in
	// JSON format including a UTC timestamp, the log level, and the
	// passed-in key/value pairs.  Or logs nothing if the corresponding log
	// level is not enabled.
	//
	// You may prefer MMap() if using common log processors.
	//
	Map(pairs ...interface{})

	// CMap() is the same as '.WithCaller(0,-1).Map(...)'.
	CMap(pairs ...interface{})

	// MMap() takes a message string followed by zero or more key/value
	// pairs.  It is the logging method that is most compatible with the
	// most log processors.  It acts like:
	//
	//      Map("msg", message, pairs...)
	//
	// except the "msg" key is taken from the Keys() config.
	//
	MMap(message string, pairs ...interface{})

	// Same as '.WithCaller(0,-1).MMap(...)'.
	CMMap(message string, pairs ...interface{})

	// With() returns a new Lager that adds to each log line the key/value
	// pairs from zero or more context.Context values.
	//
	With(ctxs ...context.Context) Lager

	// Enabled() returns 'false' only if this Lager will log nothing.
	Enabled() bool

	// WithStack() adds a "_stack" key/value pair to the logged context.  The
	// value is a list of strings where each string is a line number (base
	// 10) followed by a space and then the code file name (shortened to the
	// last 'pathParts' components).
	//
	// If 'stackLen' is 0 (or negative), then the full stack trace will be
	// included.  Otherwise, the list will contain at most 'stackLen' strings.
	// The first string will always be for depth 'minDepth'.
	//
	// A 'minDepth' of 0 starts at the line where WithStack() was called and
	// 1 starts at the line of the caller of the caller of WithStack(), etc.
	//
	// See WithCaller() for details on usage 'pathParts'.
	//
	WithStack(minDepth, stackLen, pathParts int) Lager

	// WithCaller() adds "_file" and "_line" key/value pairs to the logged
	// context.  A 'depth' of 0 means the line where WithCaller() was called,
	// and 1 is the line of the caller of the caller of WithCaller(), etc.
	//
	// 'pathParts' indicates how many directories to include in the code file
	// name.  A 0 'pathParts' includes the full path.  A 1 would only include
	// the file name.  A 2 would include the file name and the deepest sub-
	// directory name.  A -1 uses the value of lager.PathParts.
	//
	WithCaller(depth, pathParts int) Lager

	// The Println() method is provided for minimal compatibility with
	// log.Logger, as this method is the one most used by other modules.
	// It is just an alias for the List() method.
	//
	Println(...interface{})
}

// The keys to use when writing logs as a JSON map not a list.
type keyStrs struct {
	when, lev, msg, args, ctx, mod string
}

// A stub Lager that outputs nothing:
// Also used as "key" for context.Context decoration.
type noop struct{}

func (_ noop) List(_ ...interface{})             {}
func (_ noop) CList(_ ...interface{})            {}
func (_ noop) MList(_ string, _ ...interface{})  {}
func (_ noop) CMList(_ string, _ ...interface{}) {}
func (_ noop) Map(_ ...interface{})              {}
func (_ noop) CMap(_ ...interface{})             {}
func (_ noop) MMap(_ string, _ ...interface{})   {}
func (_ noop) CMMap(_ string, _ ...interface{})  {}
func (n noop) With(_ ...Ctx) Lager               { return n }
func (n noop) WithStack(_, _, _ int) Lager       { return n }
func (n noop) WithCaller(_, _ int) Lager         { return n }
func (_ noop) Enabled() bool                     { return false }
func (_ noop) Println(_ ...interface{})          {}

// The type for internal log levels.
type level int8

const (
	lPanic level = iota
	lExit
	lFail
	lWarn
	lNote
	lAcc
	lInfo
	lTrace
	lDebug
	lObj
	lGuts
	nLevels
)

// The 'logger' type is the Lager that actually logs.
type logger struct {
	lev level  // Log level
	kvp AMap   // Extra key/value pairs to append to each log line.
	mod string // The module name where the log level is en/disabled.
}

/// GLOBALS ///

// A Lager singleton for each log level (or a noop).
var _lagers [int(nLevels)]Lager

// What key strings to use (if any):
var _keys *keyStrs

// The currently enabled log levels (used by module.go).
var _enabledLevels string

// Set OutputDest to a non-nil io.Writer to not write logs to os.Stdout and
// os.Stderr.
var OutputDest io.Writer

// 'pathParts' to use when -1 is passed to WithCaller() or WithStack().
var PathParts = 0

// LevelNotation takes a log level name (like "DEBUG") and returns how that
// level should be shown in the log.  This defaults to not changing the
// level name.  If the environment variable LAGER_GCP is non-empty, then
// it instead defaults to using GcpLevelName().
var LevelNotation = func(lev string) string { return lev }

var _inGcp = os.Getenv("LAGER_GCP")

var levNames = map[level]string{
	lPanic: "PANIC",
	lExit:  "EXIT",
	lFail:  "FAIL",
	lWarn:  "WARN",
	lNote:  "NOTE",
	lAcc:   "ACCESS",
	lInfo:  "INFO",
	lTrace: "TRACE",
	lDebug: "DEBUG",
	lObj:   "OBJ",
	lGuts:  "GUTS",
}

/// FUNCS ///

func init() {
	_lagers[int(lPanic)] = &logger{lev: lPanic}
	_lagers[int(lExit)] = &logger{lev: lExit}
	Init(os.Getenv("LAGER_LEVELS"))

	if "" != _inGcp {
		RunningInGcp()
	}

	if k := os.Getenv("LAGER_KEYS"); "" != k {
		keys := strings.Split(k, ",")
		if 6 != len(keys) {
			Exit().MMap(
				"LAGER_KEYS expected 6 comma-separated labels",
				"Not", len(keys), "Value", k)
		}
		Keys(keys[0], keys[1], keys[2], keys[3], keys[4], keys[5])
	}
}

// Init() en-/disables log levels.  Pass in a string of letters from
// "FWNAITDOG" to indicate which log levels should be the only ones that
// produce output.  Each letter is the first letter of a log level (Fail,
// Warn, Note, Acc, Info, Trace, Debug, Obj, or Guts).   Levels Panic and
// Exit are always enabled.  Init("") acts like Init("FWNA"), the default
// setting.  To disable all optional logs, you can use Init("-") as any
// characters not from "FWNAITDOG" are silently ignored.  So you can also
// call Init("Fail Warn Note Access Info").
//
func Init(levels string) {
	for l := lFail; l <= lGuts; l++ {
		_lagers[int(l)] = noop{}
	}
	if "" == levels {
		levels = "FW"
	}
	enabled := make([]byte, 0, 9)
	for _, c := range levels {
		switch c {
		case 'F':
			_lagers[int(lFail)] = &logger{lev: lFail}
		case 'W':
			_lagers[int(lWarn)] = &logger{lev: lWarn}
		case 'N':
			_lagers[int(lNote)] = &logger{lev: lNote}
		case 'A':
			_lagers[int(lAcc)] = &logger{lev: lAcc}
		case 'I':
			_lagers[int(lInfo)] = &logger{lev: lInfo}
		case 'T':
			_lagers[int(lTrace)] = &logger{lev: lTrace}
		case 'D':
			_lagers[int(lDebug)] = &logger{lev: lDebug}
		case 'O':
			_lagers[int(lObj)] = &logger{lev: lObj}
		case 'G':
			_lagers[int(lGuts)] = &logger{lev: lGuts}
		default:
			continue
		}
		b := byte(c)
		if !bytes.Contains([]byte{b}, enabled) {
			enabled = append(enabled, b)
		}
	}
	_enabledLevels = string(enabled)
}

// Gets a Lager based on the internal enum for a log level.
func forLevel(lev level, cs ...Ctx) Lager {
	return _lagers[int(lev)].With(cs...)
}

// Panic() returns a Lager object that calls panic(), incorporating pairs
// from any contexts passed in.  The JSON is output to os.Stderr and then
//
//      panic("lager.Panic() logged (see above)")
//
// is called.  Holding on to the returned object may ignore future config
// updates.
//
func Panic(cs ...Ctx) Lager { return forLevel(lPanic, cs...) }

// Exit() returns a Lager object that writes to os.Stderr, incorporating
// pairs from any contexts passed in, then calls os.Exit(1).  Holding
// on to the returned object may ignore future config updates.
//
// This log level is often called "Fatal" but loggers are inconsistent as to
// whether logging at the Fatal level causes the process to exit.  By naming
// this level Exit, that ambiguity is removed.
//
// Exit() should only be used during process initialization as os.Exit()
// will prevent any 'defer'ed clean-up operations from running.
//
func Exit(cs ...Ctx) Lager { return forLevel(lExit, cs...) }

// Fail() returns a Lager object.  If the Fail log level has been disabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to report errors that are not part of the normal flow.
//
func Fail(cs ...Ctx) Lager { return forLevel(lFail, cs...) }

// Warn() returns a Lager object.  If the Warn log level has been disabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to report unusual conditions that may be signs of problems.
//
func Warn(cs ...Ctx) Lager { return forLevel(lWarn, cs...) }

// Note() returns a Lager object.  If the Note log level has been disabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to report major milestones that are part of normal flow.
//
func Note(cs ...Ctx) Lager { return forLevel(lNote, cs...) }

// Acc() returns a Lager object.  If the Acc log level has been disabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to write access logs.  The level is recorded as "ACCESS".
//
func Acc(cs ...Ctx) Lager { return forLevel(lAcc, cs...) }

// Info() returns a Lager object.  If the Info log level is not enabled, then
// the returned Lager will be one that does nothing (produces no output).
// Otherwise it incorporates pairs from any contexts passed in.  Holding on
// to the returned object may ignore future config updates.
//
// Use this to report minor milestones that are part of normal flow.
//
func Info(cs ...Ctx) Lager { return forLevel(lInfo, cs...) }

// Trace() returns a Lager object.  If the Trace log level is not enabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to trace how execution is flowing through the code.
//
func Trace(cs ...Ctx) Lager { return forLevel(lTrace, cs...) }

// Debug() returns a Lager object.  If the Debug log level is not enabled,
// then the returned Lager will be one that does nothing (produces no
// output).  Otherwise it incorporates pairs from any contexts passed in.
// Holding on to the returned object may ignore future config updates.
//
// Use this to log important details that may help in debugging.
//
func Debug(cs ...Ctx) Lager { return forLevel(lDebug, cs...) }

// Obj() returns a Lager object.  If the Obj log level is not enabled, then
// the returned Lager will be one that does nothing (produces no output).
// Otherwise it incorporates pairs from any contexts passed in.  Holding on
// to the returned object may ignore future config updates.
//
// Use this to log the details of internal data structures.
//
func Obj(cs ...Ctx) Lager { return forLevel(lObj, cs...) }

// Guts() returns a Lager object.  If the Guts log level is not enabled, then
// the returned Lager will be one that does nothing (produces no output).
// Otherwise it incorporates pairs from any contexts passed in.  Holding on
// to the returned object may ignore future config updates.
//
// Use this for debugging data that is too voluminous to always include when
// debugging.
//
func Guts(cs ...Ctx) Lager { return forLevel(lGuts, cs...) }

// Level() takes one letter from "PEFWNAITDOG" and returns a Lager object
// that either logs or doesn't, depending on whether the specified log level
// is enabled, incorporating any key/value pairs from the passed-in contexts.
// Passing in any other character calls panic().
//
func Level(lev byte, cs ...Ctx) Lager {
	switch lev {
	case 'P':
		return forLevel(lPanic, cs...)
	case 'E':
		return forLevel(lExit, cs...)
	case 'F':
		return forLevel(lFail, cs...)
	case 'W':
		return forLevel(lWarn, cs...)
	case 'N':
		return forLevel(lNote, cs...)
	case 'A':
		return forLevel(lAcc, cs...)
	case 'I':
		return forLevel(lInfo, cs...)
	case 'T':
		return forLevel(lTrace, cs...)
	case 'D':
		return forLevel(lDebug, cs...)
	case 'O':
		return forLevel(lObj, cs...)
	case 'G':
		return forLevel(lGuts, cs...)
	}
	panic(fmt.Sprintf(
		"Level() must be one char from \"PEFWNAITDOG\" not %q", lev))
}

func (l level) String() string {
	name := levNames[l]
	if "" != name {
		return name
	}
	return fmt.Sprintf("%d", int(l))
}

// Keys() provides keys to be used to write each JSON log line as a map
// (object or hash) instead of as a list (array).
//
// 'when' is used for the timestamp.  'lev' is used for the log level name.
// 'msg' is either "" or will be used for the first argument to MMap() or
// MList() (and similar methods).  'msg' is also used when a single argument
// is passed to List().  'args' is used for the arguments to List() when
// 'msg' is not.  'mod' is used for the module name (if any).
//
// 'ctx' is used for the key/value pairs added from contexts.  Specify ""
// for 'ctx' to have any context key/value pairs included in-line in the
// top-level JSON map.  In this case, care should be taken to avoid using the
// same key name both in a context pair and in a pair being passed to, for
// example, MMap().  If you do that, both pairs will be output but anything
// parsing the log line will only remember one of the pairs.
//
// If the environment variable LAGER_KEYS is set it must contain 6 key
// names separated by commas and those become the keys to use.  Otherwise, if
// the environment variable LAGER_GCP is not empty, then it is as if you had
// the following set (among other changes):
//
//      LAGER_KEYS="time,severity,message,data,,module"
//
// Pass in 6 empty strings to revert to logging a JSON list (array).
func Keys(when, lev, msg, args, ctx, mod string) {
	if "" == when && "" == lev && "" == args && "" == mod &&
		"" == ctx && "" == msg {
		_keys = nil
		return
	} else if "" == when || "" == lev || "" == args || "" == mod {
		Exit().WithCaller(1, -1).List("Only keys for msg and ctx can be blank")
	}
	_keys = &keyStrs{
		when: when, lev: lev, msg: msg, args: args, ctx: ctx, mod: mod,
	}
}

// See the Lager interface for documentation.
func (l *logger) Enabled() bool { return true }

// See the Lager interface for documentation.
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

	if nil == _keys {
		b.open("[") // ]
	} else {
		b.open("{") // }
		b.quote(_keys.when)
		b.colon()
	}
	b.timestamp()

	if nil != _keys {
		b.quote(_keys.lev)
		b.colon()
	}
	b.scalar(LevelNotation(l.lev.String()))

	return b
}

// Common closing steps for both List() and Map() methods.
func (l *logger) end(b *buffer) {
	if nil == _keys {
		b.scalar(l.kvp)
	} else if "" == _keys.ctx {
		b.pairs(l.kvp)
	} else {
		b.pair(_keys.ctx, l.kvp)
	}

	if "" != l.mod {
		if nil == _keys {
			b.quote("mod=" + l.mod)
		} else {
			b.pair(_keys.mod, l.mod)
		}
	}

	if nil == _keys { // [
		b.close("]\n")
	} else { // {
		b.close("}\n")
	}

	b.delim = ""
	b.unlock()
	bufPool.Put(b)

	switch l.lev {
	case lExit:
		os.Exit(1)
	case lPanic:
		panic("lager.Panic() logged (see above)")
	}
}

// See the Lager interface for documentation.
func (l *logger) Println(args ...interface{}) { l.List(args...) }

// See the Lager interface for documentation.
func (l *logger) List(args ...interface{}) {
	b := l.start()
	if nil == _keys {
		b.scalar(args)
	} else if 1 == len(args) && "" != _keys.msg {
		b.pair(_keys.msg, args[0])
		if "" != _inGcp && (nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	} else {
		b.pair(_keys.args, args)
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) MList(message string, args ...interface{}) {
	b := l.start()
	if nil == _keys {
		if 0 < len(args) {
			b.scalar(List(message, args))
		} else {
			// Put the single item in a list for sake of consistency:
			b.scalar(List(message))
		}
	} else if "" != _keys.msg {
		b.pair(_keys.msg, message)
		if 0 < len(args) {
			b.pair(_keys.args, args)
		} else if "" != _inGcp && (nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	} else if 0 < len(args) {
		b.pair(_keys.args, List(message, args))
	} else {
		// Put the single item in a list for sake of consistency:
		b.pair(_keys.args, List(message))
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) Map(pairs ...interface{}) {
	b := l.start()
	if nil == _keys {
		b.scalar(RawMap(pairs))
	} else {
		b.rawPairs(RawMap(pairs))
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) MMap(message string, pairs ...interface{}) {
	b := l.start()
	if nil == _keys {
		b.scalar(message)
		b.scalar(RawMap(pairs))
	} else {
		key := _keys.msg
		if "" == key {
			key = "msg"
		}
		b.pair(key, message)
		b.rawPairs(RawMap(pairs))
		if "" != _inGcp && 0 == len(pairs) &&
			(nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	}
	l.end(b)
}
