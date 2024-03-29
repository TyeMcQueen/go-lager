package lager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

/// TYPES ///

// Ctx is just an alias for context.Context that takes up less space in
// function signatures.  You never need to use lager.Ctx in your code.
type Ctx = context.Context

// Global values that are accessed via an atomic.Value so they can be safely
// initialized/updated even if somebody logs from an init() function.
//
type globals struct {
	// A Lager singleton for each log level (some will be no-ops).
	lagers [int(nLevels)]Lager

	// What key strings to use (if any).
	keys *keyStrs

	// The currently enabled log levels (used in module.go).
	enabled string

	// Optional alternate destination for logs.
	dest io.Writer

	// How much of source code file path to include in caller info.
	pathParts int

	// Required function to transform log level name before logging it.
	levDesc func(string) string

	// Add '"json": 1' when jsonPayload.text would become textPayload?
	inGcp bool

	// Used when setting Display Name of a Span.
	spanPrefix string
}

// 'Lager' is the interface returned from lager.Warn() and the other
// log-level selectors.  Of the several of its methods that can write log
// lines, MMap() is often the one you should use.
//
// For strings that contain bytes that do not form valid UTF-8, Lager will
// produce valid JSON output.  Since logs can be important in diagnosing
// problems, the non-UTF-8 parts of such strings are not replaced with the
// Unicode Replacement character ('\uFFFD') so any such characters in the
// logs indicate that '\uFFFD' was in the original string.  Instead, each
// run of non-UTF-8 bytes is replaced by a string like "«xABC0»" that will
// contain 2 base-16 digits per byte.
//
// The [C][M]Map() log-writing methods can take a list of key/value pairs
// as their final arguments.  There are special keys and types of values
// that get special handling.  The [C][M]List() log-writing methods can
// take a list of arbitrary values as their final arguments and the special
// value types apply to those as well.
//
// You can use lager.InlinePairs as a key to have a pair-containing value
// be treated as if its pairs were passed in directly.
//
// You can use a call to lager.Unless() as a key to make inclusion of that
// key/value pair optional.
//
// A value of type 'func() interface{}' will be called so its return value
// can be logged; potentially saving an expensive call when the log level
// is disabled or when lager.Unless() causes the key/value pair to be
// ignored.  [Note:  If more than about 16KiB of that log line has been
// generated before such a value is reached, then we only wait 10ms for
// the function to finish as a lock is held in that case.]
//
type Lager interface {

	// The List() method writes a single log line in JSON format including a
	// UTC timestamp, log level, and the passed-in values.  Or logs nothing
	// if the corresponding log level is not enabled.
	//
	// You may prefer MMap() if using common log processors.
	//
	List(args ...interface{})

	// CList() is the same as '.WithCaller(0).List(...)'.
	CList(args ...interface{})

	// MList() takes a message string followed by 0 or more arbitrary values.
	// Avoid interpreting values into the message string, passing them as
	// additional values instead so they can be extracted if needed.
	//
	// If Keys() have not been set, then MList() acts similar to List().  If
	// Keys() have been set, then MList() acts similar to:
	//
	//      MMap(message, "data", lager.List(args...))
	//
	// except the "data" key is taken from the Keys() config.
	//
	MList(message string, args ...interface{})

	// CMList() is the same as '.WithCaller(0).MList(...)'.
	CMList(message string, args ...interface{})

	// The Map() method takes a list of key/value pairs and writes a single
	// log line in JSON format including a UTC timestamp, the log level, and
	// the passed-in key/value pairs.  Or logs nothing if the corresponding
	// log level is not enabled.
	//
	// You may prefer MMap() if using common log processors.
	//
	Map(pairs ...interface{})

	// CMap() is the same as '.WithCaller(0).Map(...)'.
	CMap(pairs ...interface{})

	// MMap() takes a message string followed by zero or more key/value
	// pairs.  It is the logging method that is most compatible with the
	// most log processors.  It acts like:
	//
	//      Map("msg", message, pairs...)
	//
	// except the "msg" key is taken from the Keys() config.
	//
	// Avoid interpreting values into the message string, passing them
	// as additional key/value pairs instead so that they can be matched
	// or extracted more reliably.  For example:
	//
	//      // Don't do this:
	//      lager.Fail().MMap(fmt.Sprintf(
	//          "Failed connecting to %s: %v", url, err))
	//
	// is better written as:
	//
	//      lager.Fail().MMap("Failed connecting", "dest", url, "error", err)
	//
	MMap(message string, pairs ...interface{})

	// Same as '.WithCaller(0).MMap(...)'.
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
	// last 'PathParts' components) followed by a space and then the function
	// name (with package prefix stripped).
	//
	// If 'stackLen' is 0 (or negative), then the full stack trace will be
	// included.  Otherwise, the list will contain at most 'stackLen' strings.
	// The first string will always be for depth 'minDepth'.
	//
	// A 'minDepth' of 0 starts at the line where WithStack() was called and
	// 1 starts at the line of the caller of the caller of WithStack(), etc.
	//
	WithStack(minDepth, stackLen int) Lager

	// WithCaller() adds "_file", "_line", and "_func" key/value pairs to the
	// logged context.  A 'depth' of 0 means the line where WithCaller() was
	// called, and 1 is the line of the caller of the caller of WithCaller(),
	// etc.
	//
	WithCaller(depth int) Lager

	// The Println() method is provided for minimal compatibility with
	// log.Logger, as this method is the one most used by other modules.
	// It is just an alias for the List() method.
	//
	Println(...interface{})

	// LogLogger() returns a *log.Logger that uses the receiver to log
	// the constructed message.  You can pass 0 or more message filter
	// functions to modify the message before logging or to perform
	// additional actions.  If the final filter returns an empty []byte
	// value, then nothing will be logged.  A Flusher object is used as
	// the io.Writer for the created log.Logger.
	//
	LogLogger(...func(Lager, []byte) []byte) *log.Logger
}

// The keys to use when writing logs as a JSON map not a list.
type keyStrs struct {
	when, lev, msg, args, ctx, mod string
}

// A stub Lager that outputs nothing:
// Also used as "key" for context.Context decoration.
type noop struct{}

func (_ noop) List(_ ...interface{})              {}
func (_ noop) CList(_ ...interface{})             {}
func (_ noop) MList(_ string, _ ...interface{})   {}
func (_ noop) CMList(_ string, _ ...interface{})  {}
func (_ noop) Map(_ ...interface{})               {}
func (_ noop) CMap(_ ...interface{})              {}
func (_ noop) MMap(_ string, _ ...interface{})    {}
func (_ noop) CMMap(_ string, _ ...interface{})   {}
func (n noop) With(_ ...Ctx) Lager                { return n }
func (n noop) WithStack(_, _ int) Lager           { return n }
func (n noop) WithCaller(_ int) Lager             { return n }
func (_ noop) Enabled() bool                      { return false }
func (_ noop) Println(_ ...interface{})           {}

func (_ noop) LogLogger(_ ...func(Lager, []byte) []byte) *log.Logger {
	return log.New(io.Discard, "", 0)
}

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
	lev level    // Log level.
	kvp AMap     // Extra key/value pairs to append to each log line.
	mod string   // The module name where the log level is en/disabled.
	g   *globals // Global configuration at time logger was allocated.
}

// fakePanic is just used to reliably identify a panic due to lager.Exit().
type fakePanic string

/// GLOBALS ///

// Global configuration that can be updated even while logging is happening.
var _globals atomic.Value

// To ensure environment-based initialization happens when first logging is
// attempted or first code-based configuration change is made.
var _firstInit sync.Once

// Lock held when _globals is being updated.
var _globalsMutex sync.Mutex

// The special value passed to panic() [see ExitViaPanic()].
var _panicToExit = fakePanic("panic() from lager.Exit()")

// How many 'defer lager.ExitViaPanic()()' calls are waiting.
var _exiters int32 = 0

// Whether to add stack trace to all lager.Exit() logs.
var _stackWithExit int32 = 0

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

// AutoLock() can be used on any sync.Locker (anything with Lock and Unlock
// methods, like a *sync.Mutex).  Call it like:
//
//      defer lager.AutoLock(locker)()
//      //                          ^^ The 2nd set of parens is important!
//
// and the Locker will be locked immediately and unlocked when your function
// ends.
//
// If 'mu' is of type sync.Mutex, then you would have to call:
//
//      defer lager.AutoLock(&mu)()
//
// as a *sync.Mutex is a Locker but a sync.Mutex is not.
//
func AutoLock(l sync.Locker) func() {
	l.Lock()
	return l.Unlock
}

// Safely get a pointer to the current 'globals' struct.
func getGlobals() *globals {
	_firstInit.Do(firstInit)
	p := _globals.Load()
	return p.(*globals)
}

// How to safely make updates to _globals.
func updateGlobals(updater func(*globals)) {
	_firstInit.Do(firstInit)
	defer AutoLock(&_globalsMutex)()
	curr := getGlobals()
	copy := *curr
	// Copy all loggers so we can change the g pointer only in the new copies:
	for i, l := range copy.lagers {
		if pLog, ok := l.(*logger); ok {
			logCopy := *pLog
			copy.lagers[i] = &logCopy
		}
	}
	updater(&copy)
	// Update the g pointer in all loggers (after update) to the new globals:
	for _, l := range copy.lagers {
		if pLog, ok := l.(*logger); ok {
			pLog.g = &copy
		}
	}
	_globals.Store(&copy)
}

// firstInit() is called the first time logging is attempted or configuration
// changes to Lager are made via code.  It initializes configuration based
// on environment variables, making it safe to use Lager in initialization
// code.
//
func firstInit() {
	g := globals{
		pathParts: 3,
		levDesc:   identLevelNotation,
	}
	g.lagers[int(lPanic)] = &logger{lev: lPanic}
	g.lagers[int(lExit)] = &logger{lev: lExit}
	setLevels(os.Getenv("LAGER_LEVELS"))(&g)

	g.spanPrefix = os.Getenv("LAGER_SPAN_PREFIX")
	if "" == g.spanPrefix {
		parts := strings.Split(os.Args[0], "/")
		parts = strings.Split(parts[len(parts)-1], "\\")
		g.spanPrefix = parts[len(parts)-1]
	}

	// Update the g pointer in all loggers to the new globals:
	for _, l := range g.lagers {
		if pLog, ok := l.(*logger); ok {
			pLog.g = &g
		}
	}

	if "" != os.Getenv("LAGER_GCP") {
		setRunningInGcp(true)(&g)
	}

	if k := os.Getenv("LAGER_KEYS"); "" != k {
		keys := strings.Split(k, ",")
		if 6 != len(keys) {
			Exit().MMap(
				"LAGER_KEYS expected 6 comma-separated labels",
				"Not", len(keys), "Value", k)
		} else if "" == keys[0] || "" == keys[1] || "" == keys[3] ||
			"" == keys[5] {
			Exit().WithCaller(1).MMap("Only keys for msg and ctx can be blank",
				"LAGER_KEYS", keys)
		}
		setKeys(&keyStrs{
			when: keys[0], lev: keys[1], msg: keys[2],
			args: keys[3], ctx: keys[4], mod: keys[5],
		})(&g)
	}

	_globals.Store(&g)
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
// Rather than calling Init(), you may prefer to set enabled levels via the
// LAGER_LEVELS environment variable since that initialization is guaranteed
// to happen before any logging takes place, even if logging ends up being
// done in code called from initialization code.
//
func Init(levels string) {
	updateGlobals(setLevels(levels))
}

// How log level initialization is done safely.
func setLevels(levels string) func(*globals) {
	return func(g *globals) {
		for l := lFail; l <= lGuts; l++ {
			g.lagers[int(l)] = noop{}
		}
		if "" == levels {
			levels = "FWNA"
		}
		enabled := make([]byte, 0, 9)
		for _, c := range levels {
			switch c {
			case 'F':
				g.lagers[int(lFail)] = &logger{lev: lFail}
			case 'W':
				g.lagers[int(lWarn)] = &logger{lev: lWarn}
			case 'N':
				g.lagers[int(lNote)] = &logger{lev: lNote}
			case 'A':
				g.lagers[int(lAcc)] = &logger{lev: lAcc}
			case 'I':
				g.lagers[int(lInfo)] = &logger{lev: lInfo}
			case 'T':
				g.lagers[int(lTrace)] = &logger{lev: lTrace}
			case 'D':
				g.lagers[int(lDebug)] = &logger{lev: lDebug}
			case 'O':
				g.lagers[int(lObj)] = &logger{lev: lObj}
			case 'G':
				g.lagers[int(lGuts)] = &logger{lev: lGuts}
			default:
				continue
			}
			b := byte(c)
			if !bytes.Contains(enabled, []byte{b}) {
				enabled = append(enabled, b)
			}
		}
		g.enabled = string(enabled)
	}
}

// SetOutput() causes all future log lines to be written to the passed-in
// io.Writer.  If 'nil' is passed in, then log lines return to being written
// to os.Stdout (for most log levels) and to os.Stderr (for Panic and Exit
// levels).
//
// You can temporarily redirect logs via:
//
//      defer lager.SetOutput(writer)()
//      //                           ^^ Note required final parens!
//
func SetOutput(writer io.Writer) func() {
	var prior io.Writer
	updateGlobals(func(g *globals) {
		prior = g.dest
		g.dest = writer
	})
	return func() {
		updateGlobals(func(g *globals) {
			g.dest = prior
		})
	}
}

// SetPathParts() sets how many path components to include in the source
// code file names when recording caller information or a stack trace.
// Passing in 1 will cause only the source code file name to be included.
// A 2 will include the file name and the name of the directory it is in.
// A 3 adds the directory above that, etc.  A value of 0 (or -1) will include
// the full path.
//
// If you have not called SetPathParts(), it defaults to 3.
//
func SetPathParts(pathParts int) {
	updateGlobals(func(g *globals) {
		g.pathParts = pathParts
	})
}

// SetLevelNotation() installs a function to map from Lager's level names
// (like "DEBUG") to other values to indicate log levels.  An example of
// such a function is GcpLevelName().  If you write such a function, you
// would usually just key off the first letter of the passed-in level name.
//
// Passing in 'nil' for 'mapper' resets to the default behavior.
//
func SetLevelNotation(mapper func(string) string) {
	if nil == mapper {
		mapper = identLevelNotation
	}
	updateGlobals(func(g *globals) {
		g.levDesc = mapper
	})
}

func identLevelNotation(lev string) string { return lev }

// ExitViaPanic() improves the way lager.Exit() works so that uses of it
// in inappropriate places are less problematic.  Using lager.Exit() causes
// 'os.Exit(1)' to be called, which prevents any 'defer'ed code from doing
// important clean-up.  One should only use lager.Exit() in code that will
// only run during process initialization, which means it should only be
// called when there is nothing important that needs to be cleaned up.  But
// mistakes are sometimes made.
//
// Doing:
//
//      defer lager.ExitViaPanic()()
//      //                        ^^ The 2nd set of parens is important!
//
// very early in your main() function will mean that uses of lager.Exit()
// will only skip clean-up in items that were 'defer'ed before that point.
// The call to 'os.Exit(1)' will be done in the function returned from
// ExitViaPanic() rather than at the point where lager.Exit()'s return
// value was used.  See also ExitNotExpected().
//
// You can change the exit status or prevent the call to os.Exit() (such as
// for testing); see RecoverPanicToExit() for details.
//
// If you would instead like lager.Exit() to terminate the process with
// a plain panic(), then omit the 'defer' and the 2nd set of parens:
//
//      _ = lager.ExitViaPanic()
//
func ExitViaPanic() func(...func(*int)) {
	atomic.AddInt32(&_exiters, 1)
	return RecoverPanicToExit
}

// RecoverPanicToExit is the func pointer that is returned by
// ExitViaPanic().  It must be called via 'defer' and will call os.Exit(1)
// if lager.Exit() has invoked panic() because of ExitViaPanic().
//
// If you pass in one or more 'func(*int)' arguments, then they will each be
// called and passed a pointer to the exit status (initially 1) so that they
// can change it or just note the impending Exit.  If the final value is
// negative, then os.Exit() will not be called (useful when testing).
//
func RecoverPanicToExit(handlers ...func(*int)) {
	atomic.AddInt32(&_exiters, -1)
	if p := recover(); p == _panicToExit {
		exit := 1
		for _, h := range handlers {
			h(&exit)
		}
		if 0 <= exit {
			os.Exit(exit)
		}
	} else if nil != p {
		panic(p)
	}
}

// ExitNotExpected(true) causes any subsequent uses of lager.Exit() to
// include a full stack trace.  You usually call ExitNotExpected() at
// the point where process initialization has completed.  If you had not
// previously called 'defer lager.ExitViaPanic()()', then a warning is
// logged [catches accidentally writing 'defer lager.ExitViaPanic()'].
//
// ExitNotExpected(false) disables the added stack trace [and never logs
// a warning].
//
func ExitNotExpected(unexpected bool) {
	if unexpected {
		atomic.StoreInt32(&_stackWithExit, 1)
		if 0 == atomic.LoadInt32(&_exiters) {
			Warn().MMap("ExitNotExpected(true) when ExitViaPanic() not enabled",
				"cause may be", "defer lager.ExitViaPanic() // no 2nd '()'")
		}
	} else {
		atomic.StoreInt32(&_stackWithExit, 0)
	}
}

// Gets a Lager based on the internal enum for a log level.
func forLevel(lev level, cs ...Ctx) Lager {
	g := getGlobals()
	l := g.lagers[int(lev)].With(cs...)
	return l
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
// will prevent any 'defer'ed clean-up operations from running.  You can
// use ExitNotExpected() and ExitViaPanic() to find problematic uses of
// lager.Exit() and mitigate their impact.
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
	case 'P', 'p':
		return forLevel(lPanic, cs...)
	case 'E', 'e':
		return forLevel(lExit, cs...)
	case 'F', 'f':
		return forLevel(lFail, cs...)
	case 'W', 'w':
		return forLevel(lWarn, cs...)
	case 'N', 'n':
		return forLevel(lNote, cs...)
	case 'A', 'a':
		return forLevel(lAcc, cs...)
	case 'I', 'i':
		return forLevel(lInfo, cs...)
	case 'T', 't':
		return forLevel(lTrace, cs...)
	case 'D', 'd':
		return forLevel(lDebug, cs...)
	case 'O', 'o':
		return forLevel(lObj, cs...)
	case 'G', 'g':
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

// How globals.keys is updated safely.
func setKeys(keys *keyStrs) func(*globals) {
	return func(g *globals) {
		g.keys = keys
	}
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
		updateGlobals(setKeys(nil))
		return
	} else if "" == when || "" == lev || "" == args || "" == mod {
		Exit().WithCaller(1).List("Only keys for msg and ctx can be blank")
	}

	updateGlobals(setKeys(&keyStrs{
		when: when, lev: lev, msg: msg, args: args, ctx: ctx, mod: mod,
	}))
}

// GetSpanPrefix() returns a string to be used as the prefix for the Display
// Name of trace spans.  It defaults to os.Getenv("LAGER_SPAN_PREFIX") or,
// if that is not set, to the basename of 'os.Args[0]'.
//
func GetSpanPrefix() string {
	return getGlobals().spanPrefix
}

// SetSpanPrefix() sets the span name prefix [see GetSpanPrefix()].
//
func SetSpanPrefix(prefix string) {
	updateGlobals(func(g *globals) {
		g.spanPrefix = prefix
	})
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

// Opening steps when actually logging a line.
func (l *logger) start() *buffer {
	b := bufPool.Get().(*buffer)
	b.g = l.g
	switch l.lev {
	case lPanic, lExit:
		b.w = os.Stderr
	default:
		b.w = os.Stdout
	}
	if nil != b.g.dest {
		b.w = b.g.dest
	}

	if nil == l.g.keys {
		b.open("[") // ]
	} else {
		b.open("{") // }
		b.quote(l.g.keys.when)
		b.colon()
	}
	b.timestamp()

	if nil != l.g.keys {
		b.quote(l.g.keys.lev)
		b.colon()
	}
	b.scalar(b.g.levDesc(l.lev.String()))

	return b
}

// Closing steps when actually logging a line.
func (l *logger) end(b *buffer) {
	if lExit == l.lev && 0 != atomic.LoadInt32(&_stackWithExit) {
		// 0: skip end(), 1: skip MMap() etc, 2: get caller of MMap() etc:
		l = l.WithStack(2, 0).(*logger)
	}
	if nil != l.kvp && 0 < len(l.kvp.keys) {
		if nil == l.g.keys {
			b.scalar(l.kvp)
		} else if "" == l.g.keys.ctx {
			b.pairs(l.kvp)
		} else {
			b.pair(l.g.keys.ctx, l.kvp)
		}
	}

	if "" != l.mod {
		if nil == l.g.keys {
			b.quote("mod=" + l.mod)
		} else {
			b.pair(l.g.keys.mod, l.mod)
		}
	}

	if nil == l.g.keys { // [
		b.close("]\n")
	} else { // {
		b.close("}\n")
	}

	b.delim = ""
	b.unlock()
	bufPool.Put(b)

	switch l.lev {
	case lExit:
		if 0 == atomic.LoadInt32(&_exiters) {
			os.Exit(1)
		}
		panic(_panicToExit)
	case lPanic:
		panic("lager.Panic() logged (see above)")
	}
}

// See the Lager interface for documentation.
func (l *logger) Println(args ...interface{}) { l.List(args...) }

// See the Lager interface for documentation.
func (l *logger) LogLogger(filters ...func(Lager, []byte) []byte) *log.Logger {
	return log.New(Flusher{l, filters}, "", 0)
}

// See the Lager interface for documentation.
func (l *logger) List(args ...interface{}) {
	b := l.start()
	if nil == l.g.keys {
		if 0 == len(args) {
			b.write(", []")
		} else if 1 == len(args) {
			b.scalar(args[0])
		} else {
			b.scalar(args)
		}
	} else if 1 == len(args) && "" != l.g.keys.msg {
		b.pair(l.g.keys.msg, args[0])
		if l.g.inGcp && (nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	} else {
		b.pair(l.g.keys.args, args)
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) MList(message string, args ...interface{}) {
	b := l.start()
	if nil == l.g.keys {
		if 0 == len(args) {
			b.scalar(message)
		} else {
			b.msgList(message, args)
		}
	} else if "" != l.g.keys.msg {
		b.pair(l.g.keys.msg, message)
		if 0 < len(args) {
			b.pair(l.g.keys.args, args)
		} else if l.g.inGcp && (nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	} else if 0 < len(args) {
		b.quote(l.g.keys.args)
		b.colon()
		b.msgList(message, args)
	} else {
		// Put the single item in a list for sake of consistency:
		b.pair(l.g.keys.args, List(message))
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) Map(pairs ...interface{}) {
	b := l.start()
	if nil == l.g.keys {
		b.scalar(RawMap(pairs))
	} else {
		b.rawPairs(RawMap(pairs))
	}
	l.end(b)
}

// See the Lager interface for documentation.
func (l *logger) MMap(message string, pairs ...interface{}) {
	b := l.start()
	if nil == l.g.keys {
		b.scalar(message)
		if 0 < len(pairs) {
			b.scalar(RawMap(pairs))
		}
	} else {
		key := l.g.keys.msg
		if "" == key {
			key = "msg"
		}
		b.pair(key, message)
		b.rawPairs(RawMap(pairs))
		if l.g.inGcp && 0 == len(pairs) &&
			(nil == l.kvp || 0 == len(l.kvp.keys)) {
			b.pair("json", 1) // Keep jsonPayload.message not textPayload
		}
	}
	l.end(b)
}
