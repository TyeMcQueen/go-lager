package lager

import(
	"fmt"
	"os"
	"strconv"
	"sync"
)


// A named module that allows separate log levels to be en-/disabled.
type Module struct {
	name   string
	levels string
	lagers [int(nLevels)]Lager
}


var modMap sync.Map


func getMod(name string) *Module {
	x, ok := modMap.Load(name)
	if ! ok {
		return nil  // No such module
	}
	if mod, ok := x.(*Module); ok {
		return mod  // Valid module (or valid nil pointer of type *Module)
	}
	return nil      // Invalid module got stored somehow?
}

func storeMod(name string, mod *Module) *Module {
	_, _ = modMap.LoadOrStore(name, mod)
	cur := getMod(name)
	if nil == cur {             // An invalid module got stored somehow:
		modMap.Store(name, mod) // Overwrite it.
		cur = getMod(name)
		if nil == cur {         // Stored module is still invalid:
			panic("Failed to store module " + name)
		}
	}
	return cur
}

// En-/disables log levels for the named module.  If no module by that name
// exists yet, then false is returned.
func SetModuleLevels(name, levels string) bool {
	mod := getMod(name)
	if nil == mod {
		return false
	}
	mod.Init(levels)
	return true
}

// En-/disables log levels for the named module.  If no module by that name
// exists yet, then "n/a" is returned.  Otherwise returns the enabled levels.
func GetModuleLevels(name string) string {
	mod := getMod(name)
	if nil == mod {
		return "n/a"
	}
	return mod.levels
}

// Returns a map[string]string where the keys are all of the module names and
// the values are their enabled levels.
func GetModules() map[string]string {
	m := make(map[string]string)
	modMap.Range(func(key, value interface{}) bool {
		m[key.(string)] = value.(*Module).levels
		return true
	})
	return m
}

// Create a new Module with the given name.  Default log levels can also be
// passed in as an optional second argument.  The initial log levels enabled
// are taken from the last item in the list that is not "":
//    The current globally enabled levels.
//    The (optional) passed-in defaultLevels.
//    The value of the LAGER_{module_name}_LEVELS environment variable.
// If you wish to ignore the LAGER_{module_name}_LEVELS environment varible,
// then write code similar to:
//    mod := lager.NewModule("mymod").Init("FW")
func NewModule(name string, defaultLevels ...string) *Module {
	mod := getMod(name)
	if nil != mod {
		return mod
	}
	mod = &Module{name: name}
	levels := ""
	if 1 == len(defaultLevels) {
		levels = defaultLevels[0]
	} else if 0 != len(defaultLevels) {
		panic("Passed more than one defaultLevel string to lager.NewModule()")
	}
	env := os.Getenv("LAGER_" + name + "_LEVELS")
	if "" != env {
		levels = env
	}
	mod.Init(levels)
	return storeMod(name, mod)
}

// En-/disables log levels.  Pass in a string of letters from "FWITDOG" to
// indicate which log levels should be the only ones that produce output.
// Each letter is the first letter of a log level (Fail, Warn, Info, Trace,
// Debug, Obj, or Guts).   Levels Panic and Exit are always enabled.  Init("")
// copies the current globally enabled levels.  To disable all optional logs,
// you can use Init("-") as any characters not from "FWITDOG" are silently
// ignored.  So you can also call Init("Fail Warn Info").
func (m *Module) Init(levels string) *Module {
	m.levels = ""
	for l := lFail; l <= lGuts; l++ {
		m.lagers[int(l)] = noop{}
	}
	if "" == levels {
		levels = _enabledLevels
	}
	for _, c := range levels {
		switch c {
		case 'F': m.lagers[int(lFail)]  = &logger{lev: lFail,  mod: m.name}
		case 'W': m.lagers[int(lWarn)]  = &logger{lev: lWarn,  mod: m.name}
		case 'I': m.lagers[int(lInfo)]  = &logger{lev: lInfo,  mod: m.name}
		case 'T': m.lagers[int(lTrace)] = &logger{lev: lTrace, mod: m.name}
		case 'D': m.lagers[int(lDebug)] = &logger{lev: lDebug, mod: m.name}
		case 'O': m.lagers[int(lObj)]   = &logger{lev: lObj,   mod: m.name}
		case 'G': m.lagers[int(lGuts)]  = &logger{lev: lGuts,  mod: m.name}
		default:  continue
		}
		m.levels += strconv.QuoteRune(c)
	}
	return m
}

func (m *Module) modLevel(lev level, cs ...Ctx) Lager {
	return m.lagers[int(lev)].With(cs...)
}

// Returns a Lager object that calls panic().  The JSON log line is first
// output to os.Stderr and then
//    panic("lager.Panic() logged (see above)")
// is called.
func (m *Module) Panic(cs ...Ctx) Lager { return m.modLevel(lPanic, cs...) }

// Returns a Lager object that writes to os.Stderr then calls os.Exit(1).
// This log level is often called "Fatal" but loggers are inconsistent as to
// whether logging at the Fatal level causes the process to exit.  By naming
// this level Exit, that ambiguity is removed.
func (m *Module) Exit(cs ...Ctx) Lager { return m.modLevel(lExit, cs...) }

// Returns a Lager object.  If Fail log level has been disabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report errors that are not part of the normal flow.
func (m *Module) Fail(cs ...Ctx) Lager { return m.modLevel(lFail, cs...) }

// Returns a Lager object.  If Warn log level has been disabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report unusual conditions that may be signs of problems.
func (m *Module) Warn(cs ...Ctx) Lager { return m.modLevel(lWarn, cs...) }

// Returns a Lager object.  If Info log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to report major milestones that are part of normal flow.
func (m *Module) Info(cs ...Ctx) Lager { return m.modLevel(lInfo, cs...) }

// Returns a Lager object.  If Trace log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to trace how execution is flowing through the code.
func (m *Module) Trace(cs ...Ctx) Lager { return m.modLevel(lTrace, cs...) }

// Returns a Lager object.  If Debug log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to log important details that may help in debugging.
func (m *Module) Debug(cs ...Ctx) Lager { return m.modLevel(lDebug, cs...) }

// Returns a Lager object.  If Obj log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level to log the details of internal data structures.
func (m *Module) Obj(cs ...Ctx) Lager { return m.modLevel(lObj, cs...) }

// Returns a Lager object.  If Guts log level is not enabled, then the
// returned Lager will be one that does nothing (produces no output).  Use
// this log level for debugging data that is too voluminous to always include
// when debugging.
func (m *Module) Guts(cs ...Ctx) Lager { return m.modLevel(lGuts, cs...) }

// Pass in one character from "PEFWITDOG" to get a Lager object that either
// logs or doesn't, depending on whether the specified log level is enabled.
func (m *Module) Level(lev byte, cs ...Ctx) Lager {
	switch lev {
	case 'P': return m.modLevel(lPanic, cs...)
	case 'E': return m.modLevel(lExit, cs...)
	case 'F': return m.modLevel(lFail, cs...)
	case 'W': return m.modLevel(lWarn, cs...)
	case 'I': return m.modLevel(lInfo, cs...)
	case 'T': return m.modLevel(lTrace, cs...)
	case 'D': return m.modLevel(lDebug, cs...)
	case 'O': return m.modLevel(lObj, cs...)
	case 'G': return m.modLevel(lGuts, cs...)
	}
	panic(fmt.Sprintf(
		"Level() must be one char from \"PEFWITDOG\" not %q", lev))
}
