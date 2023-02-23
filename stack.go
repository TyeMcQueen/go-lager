package lager

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

var _pathSep = string(os.PathSeparator)

func caller(depth, pathparts int) (file string, line int, funcname string) {
	pcs := make([]uintptr, 1)
	if n := runtime.Callers(3+depth, pcs); n < 1 {
		return
	}
	frame, _ := runtime.CallersFrames(pcs).Next()
	if 0 == frame.PC {
		return
	}
	file, line, funcname = frame.File, frame.Line, frame.Function

	if -1 == pathparts {
		pathparts = PathParts
	}
	if fnparts := strings.Split(funcname, "."); 0 < len(fnparts) {
		funcname = fnparts[len(fnparts)-1]
	}
	if 0 < pathparts {
		parts := strings.Split(file, _pathSep)
		if pathparts < len(parts) {
			l := len(parts)
			file = strings.Join(parts[l-pathparts:l], _pathSep)
		}
	}
	return file, line, funcname
}

// See the Lager interface for documentation.
func (l *logger) WithCaller(depth int) Lager {
	file, line, fn := caller(depth, -1)
	if 0 == line {
		return l
	}
	cp := *l
	cp.kvp = cp.kvp.Merge(Pairs("_file", file, "_line", line, "_func", fn))
	return &cp
}

// See the Lager interface for documentation.
func (l *logger) WithStack(minDepth, stackLen int) Lager {
	stack := make([]string, 0)
	for depth := minDepth; true; depth++ {
		if 0 < stackLen && stackLen <= depth-minDepth {
			break
		}
		file, line, fn := caller(depth, -1)
		if 0 == line {
			break
		}
		if "" == fn {
			stack = append(stack, fmt.Sprintf("%d %s", line, file))
		} else {
			stack = append(stack, fmt.Sprintf("%d %s %s", line, file, fn))
		}
	}
	cp := *l
	cp.kvp = cp.kvp.Merge(Pairs("_stack", stack))
	return &cp
}

// See the Lager interface for documentation.
func (l *logger) CList(args ...interface{}) {
	l.WithCaller(1).List(args...)
}

// See the Lager interface for documentation.
func (l *logger) CMList(message string, args ...interface{}) {
	l.WithCaller(1).MList(message, args...)
}

// See the Lager interface for documentation.
func (l *logger) CMap(args ...interface{}) {
	l.WithCaller(1).Map(args...)
}

// See the Lager interface for documentation.
func (l *logger) CMMap(message string, args ...interface{}) {
	l.WithCaller(1).MMap(message, args...)
}
