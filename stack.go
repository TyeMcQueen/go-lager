package lager

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

var _pathSep = string(os.PathSeparator)

func caller(depth, pathparts int) (string, int) {
	_, file, line, ok := runtime.Caller(2 + depth)
	if !ok {
		return "", 0
	}
	if -1 == pathparts {
		pathparts = PathParts
	}
	if 0 < pathparts {
		parts := strings.Split(file, _pathSep)
		if pathparts < len(parts) {
			l := len(parts)
			file = strings.Join(parts[l-pathparts:l], _pathSep)
		}
	}
	return file, line
}

// See the Lager interface for documentation.
func (l *logger) WithCaller(depth int, pathparts ...int) Lager {
	parts := -1
	if 0 < len(pathparts) {
		parts = pathparts[0]
	}
	file, line := caller(depth, parts)
	if 0 == line {
		return l
	}
	cp := *l
	cp.kvp = cp.kvp.Merge(Pairs("_file", file, "_line", line))
	return &cp
}

// See the Lager interface for documentation.
func (l *logger) WithStack(minDepth, stackLen int, pathparts ...int) Lager {
	parts := -1
	if 0 < len(pathparts) {
		parts = pathparts[0]
	}
	stack := make([]string, 0)
	for depth := minDepth; true; depth++ {
		if 0 < stackLen && stackLen <= depth-minDepth {
			break
		}
		file, line := caller(depth, parts)
		if 0 == line {
			break
		}
		stack = append(stack, fmt.Sprintf("%d %s", line, file))
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
