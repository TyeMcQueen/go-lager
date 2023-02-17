package buffer

import (
	"bytes"
	"strings"
	"sync"
)

// AsyncBuilder is like strings.Builder but it can safely be used from
// multiple goroutines at once.  It is useful as a logging destination
// inside of automated tests if you only ever want the entire recent
// log contents [since the last Reset()] as a single string.
//
// 'new(buffer.AsyncBuilder)' should be used to create an AsyncBuilder.
// You can also just declare a buffer.AsyncBuilder so long as you are
// careful to never make a copy of it.
//
type AsyncBuilder struct {
	mu sync.Mutex
	bu strings.Builder
}

// AsyncBuffer is like bytes.Buffer but it can safely be used from multiple
// goroutines at once.  But it does not implement every bytes.Buffer method.
// It adds ReadAll() and ReadAllString() methods for atomic operations.
// It is useful as a logging destination inside of automated tests.
//
// 'new(buffer.AsyncBuffer)' should be used to create an AsyncBuffer.
// You can also just declare a buffer.AsyncBuffer so long as you are
// careful to never make a copy of it.
//
type AsyncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (ab *AsyncBuffer) lock() func() {
	ab.mu.Lock()
	return ab.mu.Unlock
}

func (ab *AsyncBuffer) Write(p []byte) (int, error) {
	defer ab.lock()()
	return ab.buf.Write(p)
}

func (ab *AsyncBuffer) Reset() {
	defer ab.lock()()
	ab.buf.Reset()
}

// ReadAll() is similar to Bytes() except it also atomically Reset()s the
// buffer and the returned slice is a copy that is not changed by subsequent
// buffer operations.
//
func (ab *AsyncBuffer) ReadAll() []byte {
	defer ab.lock()()
	ret := make([]byte, ab.buf.Len())
	copy(ret, ab.buf.Bytes())
	ab.buf.Reset()
	return ret
}

// ReadAllString() is similar to String() except it also atomically Reset()s
// the buffer.
//
func (ab *AsyncBuffer) ReadAllString() string {
	defer ab.lock()()
	ret := ab.buf.String()
	ab.buf.Reset()
	return ret
}

func (ab *AsyncBuffer) Bytes() []byte {
	defer ab.lock()()
	return ab.buf.Bytes()
}

func (ab *AsyncBuffer) String() string {
	defer ab.lock()()
	return ab.buf.String()
}

func (ab *AsyncBuffer) Len() int {
	defer ab.lock()()
	return ab.buf.Len()
}

func (ab *AsyncBuffer) Truncate(n int) {
	defer ab.lock()()
	ab.buf.Truncate(n)
}

func (ab *AsyncBuffer) ReadString(delim byte) (string, error) {
	defer ab.lock()()
	return ab.buf.ReadString(delim)
}

func (ab *AsyncBuffer) ReadBytes(delim byte) ([]byte, error) {
	defer ab.lock()()
	return ab.buf.ReadBytes(delim)
}


func (sb *AsyncBuilder) lock() func() {
	sb.mu.Lock()
	return sb.mu.Unlock
}

func (sb *AsyncBuilder) Grow(n int) {
	defer sb.lock()()
	sb.bu.Grow(n)
}

func (sb *AsyncBuilder) Len() int {
	defer sb.lock()()
	return sb.bu.Len()
}

func (sb *AsyncBuilder) Reset() {
	defer sb.lock()()
	sb.bu.Reset()
}

func (sb *AsyncBuilder) String() string {
	defer sb.lock()()
	return sb.bu.String()
}

// ReadAll() is like String() but it also atomically Reset()s the builder.
//
func (sb *AsyncBuilder) ReadAll() string {
	defer sb.lock()()
	ret := sb.bu.String()
	sb.bu.Reset()
	return ret
}

func (sb *AsyncBuilder) Write(p []byte) (int, error) {
	defer sb.lock()()
	return sb.bu.Write(p)
}

func (sb *AsyncBuilder) WriteByte(c byte) error {
	defer sb.lock()()
	return sb.bu.WriteByte(c)
}

func (sb *AsyncBuilder) WriteRune(r rune) (int, error) {
	defer sb.lock()()
	return sb.bu.WriteRune(r)
}

func (sb *AsyncBuilder) WriteString(s string) (int, error) {
	defer sb.lock()()
	return sb.bu.WriteString(s)
}
