package lager

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func caller(depth, pathparts int) (string, int) {
	_, file, line, ok := runtime.Caller(2 + depth)
	if !ok {
		return "", 0
	}
	if -1 == pathparts {
		pathparts = PathParts
	}
	if 0 < pathparts {
		parts := filepath.SplitList(file)
		if pathparts < len(parts) {
			l := len(parts)
			file = filepath.Join(parts[l-pathparts : l]...)
		}
	}
	return file, line
}

// See the Lager interface for documentation.
func (l *logger) WithCaller(depth, pathparts int) Lager {
	file, line := caller(depth, pathparts)
	if 0 == line {
		return l
	}
	cp := *l
	cp.kvp = cp.kvp.Merge(Pairs("_file", file, "_line", line))
	return &cp
}

// See the Lager interface for documentation.
func (l *logger) WithStack(minDepth, stackLen, pathparts int) Lager {
	stack := make([]string, 0)
	for depth := minDepth; true; depth++ {
		if 0 < stackLen && stackLen <= depth-minDepth {
			break
		}
		file, line := caller(depth, pathparts)
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
	l.WithCaller(1, -1).List(args...)
}

// See the Lager interface for documentation.
func (l *logger) CMList(message string, args ...interface{}) {
	l.WithCaller(1, -1).MList(message, args...)
}

// See the Lager interface for documentation.
func (l *logger) CMap(args ...interface{}) {
	l.WithCaller(1, -1).Map(args...)
}

// See the Lager interface for documentation.
func (l *logger) CMMap(message string, args ...interface{}) {
	l.WithCaller(1, -1).MMap(message, args...)
}
