package lager

// Low-level code for composing a log line.

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"
)

/// TYPES ///

// An unshared, temporary structure for efficiently logging one line.
type buffer struct {
	scratch [16 * 1024]byte // Space so we can allocate memory only rarely.
	buf     []byte          // Bytes not yet written (a slice into above).
	w       io.Writer       // Usually os.Stdout, else os.Stderr.
	delim   string          // Delimiter to go before next value.
	locked  bool            // Whether we had to lock outMu.
	g       *globals
}

// A Stringer just has a String() method that returns its stringification.
type Stringer interface {
	String() string
}

/// GLOBALS ///

// Minimize how many of these must be allocated:
var bufPool = sync.Pool{New: func() interface{} {
	b := new(buffer)
	b.buf = b.scratch[0:0]
	return b
}}

// A lock in case a log line is too large to buffer.
var outMu sync.Mutex

// The (JSON) delimiter between values:
const comma = ", "

/// FUNCS ///

var noEsc [256]bool
var hexDigits = "0123456789abcdef"

func init() {
	for c := ' '; c < 256; c++ {
		noEsc[c] = true
	}
	noEsc['"'] = false
	noEsc['\\'] = false
}

// Called when we need to flush early, to prevent interleaved log lines.
func (b *buffer) lock() {
	if !b.locked {
		outMu.Lock()
		b.locked = true
	}
	if 0 < len(b.buf) {
		b.w.Write(b.buf)
		b.buf = b.scratch[0:0]
	}
}

// Called when finished composing a log line.
func (b *buffer) unlock() {
	if 0 < len(b.buf) {
		b.w.Write(b.buf)
		b.buf = b.scratch[0:0]
	}
	if b.locked {
		b.locked = false
		outMu.Unlock()
	}
}

// Append a slice of bytes to the log line.
func (b *buffer) writeBytes(s []byte) {
	if cap(b.buf) < len(b.buf)+len(s) {
		b.lock() // Can't fit line in buffer; lock output mutex and flush.
	}
	if cap(b.buf) < len(s) {
		b.w.Write(s) // Next chunk won't fit in buffer, just write it.
	} else {
		b.buf = append(b.buf, s...)
	}
}

// Append strings to the log line.
func (b *buffer) write(strs ...string) {
	for _, s := range strs {
		if cap(b.buf) < len(b.buf)+len(s) {
			b.lock()
		}
		if cap(b.buf) < len(s) {
			io.WriteString(b.w, s)
		} else {
			was := len(b.buf)
			will := was + len(s)
			b.buf = b.buf[0:will]
			for i := 0; i < len(s); i++ {
				b.buf[was+i] = s[i]
			}
		}
	}
}

func escapeByte(c byte) string {
	switch c {
	case '"':
		return "\\\""
	case '\\':
		return "\\\\"
	case '\b':
		return "\\b"
	case '\f':
		return "\\f"
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	}
	buf := []byte("\\u0000")
	for o := 5; 1 < o; o-- {
		h := c & 0xF
		buf[o] = hexDigits[h]
		c >>= 4
	}
	return string(buf)
}

// Append a quoted (JSON) string to the log line.  If more than one string
// is passed in, then they are concatenated together.
func (b *buffer) quote(strs ...string) {
	b.write(b.delim, `"`)
	for _, s := range strs {
		b.escape(s)
	}
	b.write(`"`)
	b.delim = comma
}

// Append a quoted (JSON) string (from a byte slice) to the log line.
func (b *buffer) quoteBytes(s []byte) {
	b.write(b.delim, `"`)
	b.escapeBytes(s)
	b.write(`"`)
}

// Append an escaped string as part of a quoted JSON string.
func (b *buffer) escape(s string) {
	beg := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if noEsc[c] {
			continue
		}
		b.write(s[beg:i])
		b.write(escapeByte(c))
		beg = i + 1
	}
	b.write(s[beg:])
}

// Append an escaped string (from a byte slice), part of a quoted JSON string.
func (b *buffer) escapeBytes(s []byte) {
	beg := 0
	for i, c := range s {
		if noEsc[c] {
			continue
		}
		b.writeBytes(s[beg:i])
		b.write(escapeByte(c))
		beg = i + 1
	}
	b.writeBytes(s[beg:])
}

// Append a 2-digit value to the buffer (with leading '0').
func (b *buffer) int2(val int) {
	// Not needed so long as calls to int2() remain protected:
	//  if cap(b.buf) < len(b.buf) + 2 {
	//      b.lock()
	//  }
	l := len(b.buf)
	b.buf = b.buf[0 : 2+l]
	b.buf[l] = '0' + byte(val/10)
	b.buf[l+1] = '0' + byte(val%10)
}

// Append a decimal value of specified length with leading '0's.
func (b *buffer) int(val int, digits int) {
	// Not needed so long as calls to int() remain protected:
	//  if cap(b.buf) < len(b.buf) + digits {
	//      b.lock()
	//  }
	bef := len(b.buf)
	b.buf = strconv.AppendInt(b.buf, int64(val), 10)
	aft := len(b.buf)
	l := aft - bef
	// Prepend leading '0's to get desired length:
	if l < digits {
		b.buf = b.buf[0 : bef+digits]
		copy(b.buf[bef+digits-l:bef+digits], b.buf[bef:aft])
		for i := bef; i < bef+digits-l; i++ {
			b.buf[i] = '0'
		}
	}
}

// Append a quoted UTC timestamp to the log line.
func (b *buffer) timestamp() {
	if cap(b.buf) < len(b.buf)+22 {
		b.lock()
	}
	now := time.Now().In(time.UTC)
	b.write(`"`)
	yr, mo, day := now.Date()
	b.buf = strconv.AppendInt(b.buf, int64(yr), 10)
	b.write("-")
	b.int2(int(mo))
	b.write("-")
	b.int2(day)
	if nil == b.g.keys {
		b.write(" ") // Use easier-for-humans-to-read format
	} else {
		b.write("T") // Use standard format (GCP cares)
	}
	b.int2(now.Hour())
	b.write(":")
	b.int2(now.Minute())
	b.write(":")
	b.int2(now.Second())
	b.write(".")
	b.int(now.Nanosecond()/100000, 4)
	b.write(`Z"`)
	b.delim = comma
}

// Begin appending a nested data structure to the log line.
func (b *buffer) open(punct string) {
	b.write(b.delim, punct)
	b.delim = ""
}

// Append the key/value separator ":" to the log line.
func (b *buffer) colon() {
	b.write(":")
	b.delim = ""
}

// End appending a nested data structure to the log line.
func (b *buffer) close(punct string) {
	b.write(punct)
	b.delim = comma
}

// Append a single key/value pair:
func (b *buffer) pair(k string, v interface{}) {
	b.quote(k)
	b.colon()
	b.scalar(v)
}

// Append the key/value pairs from AMap:
func (b *buffer) pairs(m AMap) {
	if nil != m {
		for i, k := range m.keys {
			b.pair(k, m.vals[i])
		}
	}
}

// Append the key/value pairs from a RawMap:
func (b *buffer) rawPairs(m RawMap) {
	skipping := false
	inlining := false
	for i, elt := range m {
		if 0 == 1&i {
			if _, ok := elt.(skipThisPair); ok {
				skipping = true
			} else if _, ok := elt.(inlinePairs); ok {
				inlining = true
			} else {
				b.quote(S(elt))
				b.colon()
			}
		} else if skipping {
			skipping = false
		} else if inlining {
			switch m := elt.(type) {
			case RawMap:
				b.rawPairs(m)
			case KVPairs:
				b.pairs(&m)
			case AMap:
				b.pairs(m)
			default:
				b.pair("cannot-inline", elt)
			}
			inlining = false
		} else {
			b.scalar(elt)
		}
	}
	if 1 == 1&len(m) && !skipping {
		b.scalar(nil)
	}
}

// Append a JSON-encoded scalar value to the log line.
func (b *buffer) scalar(s interface{}) {
	switch v := s.(type) {
	case func() interface{}:
		s = v()
	}
	switch v := s.(type) {
	case AMap:
		if nil == v || 0 == len(v.keys) {
			return
		}
	}
	b.write(b.delim)
	b.delim = ""
	if cap(b.buf) < len(b.buf)+64 {
		b.lock() // Leave room for strconv.AppendFloat() or similar
	}
	switch v := s.(type) {
	case nil:
		b.write("null")
	case string:
		b.quote(v)
	case []byte:
		b.quoteBytes(v)
	case int:
		b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int8:
		b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int16:
		b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int32:
		b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int64:
		b.buf = strconv.AppendInt(b.buf, v, 10)
	case uint:
		b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint8:
		b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint16:
		b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint32:
		b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint64:
		b.buf = strconv.AppendUint(b.buf, v, 10)
	case float32:
		b.buf = strconv.AppendFloat(b.buf, float64(v), 'g', -1, 32)
	case float64:
		b.buf = strconv.AppendFloat(b.buf, v, 'g', -1, 64)
	case bool:
		if v {
			b.write("true")
		} else {
			b.write("false")
		}
	case []string:
		b.open("[")
		for _, s := range v {
			b.scalar(s)
		}
		b.close("]")
	case AList:
		b.open("[")
		for _, s := range v {
			b.scalar(s)
		}
		b.close("]")
	case RawMap:
		b.open("{")
		b.rawPairs(v)
		b.close("}")
	case AMap:
		b.open("{")
		b.pairs(v)
		b.close("}")
	case map[string]interface{}:
		keys := make([]string, len(v))
		i := 0
		for k, _ := range v {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		b.open("{")
		for _, k := range keys {
			b.pair(k, v[k])
		}
		b.close("}")
	case error:
		b.quote(v.Error())
	case Stringer:
		b.quote(v.String())
	default:
		buf, err := json.Marshal(v)
		if nil != err {
			b.quote("! ", err.Error(), "; ", fmt.Sprintf("%#v", v))
		} else {
			b.writeBytes(buf)
		}
	}
	b.delim = comma
}
