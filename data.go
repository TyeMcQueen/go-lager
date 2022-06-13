package lager

import (
	"context"
	"fmt"
)

type skipThisPair string

// SkipThisPair can be used as a "label" to indicate that the following
// value should just be ignored.  You would usually call lager.Unless()
// rather than use this directly.
//
const SkipThisPair = skipThisPair("")

type inlinePairs string

// InlinePairs can be used as a "label" to indicate that the following
// value that contains label-subvalue pairs (a value of type AMap or RawMap)
// should be treated as if the pairs had been passed in at that higher level.
//
//      func Assert(pairs ...interface{}) {
//          lager.Fail().MMap("Assertion failed", lager.InlinePairs, pairs)
//      }
//
const InlinePairs = inlinePairs("")

// Storage for an ordered list of key/value pairs (without duplicate keys).
type KVPairs struct {
	keys []string
	vals []interface{}
}

// A list type that we efficiently convert to JSON.
type AList = []interface{}

// A raw list of key/value pairs we can efficiently convert to JSON as a map.
type RawMap []interface{}

// A processed list of key/value pairs we can efficiently convert to JSON.
type AMap = *KVPairs

// S() converts an arbitrary value to a string.  It is very similar to
// 'fmt.Sprintf("%v", arg)' but treats []byte values the same as strings
// rather then dumping them as a list of byte values in base 10.
//
func S(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return fmt.Sprintf("%v", arg)
}

// lager.List() returns a slice (lager.AList) that can be passed as an
// argument to a Lager's [C][M]Map() or [C][M]List() method to construct
// nested data that can be quickly serialized to JSON.  For example:
//
//      lager.Info().Map("User", u, "not in", lager.List(one, two, three))
//
func List(args ...interface{}) AList { return args }

// lager.Map() returns a raw list of key/value pairs (lager.RawMap) that can
// be passed as an argument to a Lager's [C][M]Map() or [C][M]List() method
// to construct nested data that can be quickly serialized to JSON.  I.e.:
//
//      lager.Info().List("Using", lager.Map("name", name, "age", age))
//
// Dupliate keys all get output.  If you need to squash duplicate keys, then
// call lager.Pairs() instead.
//
func Map(pairs ...interface{}) RawMap { return RawMap(pairs) }

// lager.Pairs() returns a (processed) list of key/value pairs (lager.AMap)
// that can be added to a context.Context to specify additional data to
// append to each log line.  It can also be used similar to lager.Map().
//
func Pairs(pairs ...interface{}) AMap {
	return AMap(nil).AddPairs(pairs...)
}

// Unless() is used to pass an optional label+value pair to Map().  Use
// Unless() to specify the label and, if the value is unsafe or expensive to
// compute, then wrap it in a deferring function:
//
//      lager.Debug().Map(
//          "Ran", stage,
//          // Don't include `"Error": null,` in the log:
//          lager.Unless(nil == err, "Error"), err,
//          // Don't call config.Proxy() if config is nil:
//          lager.Unless(nil == config, "Proxy"),
//              func() interface{} { return config.Proxy() },
//      )
//
func Unless(cond bool, label string) interface{} {
	if !cond {
		return label
	}
	return SkipThisPair
}

// Add/update Lager key/value pairs to/in a context.Context.
func AddPairs(ctx Ctx, pairs ...interface{}) Ctx {
	if 0 == len(pairs) {
		return ctx
	}
	return ContextPairs(ctx).AddPairs(pairs...).InContext(ctx)
}

// Fetches the lager key/value pairs stored in a context.Context.
func ContextPairs(ctx Ctx) AMap {
	x := ctx.Value(noop{})
	switch v := x.(type) {
	case nil:
		return nil
	case AMap:
		return v
	}
	panic(fmt.Sprintf("Invalid type (%T not *lager.KVPairs) in context", x))
}

// Get a new context with this map stored in it.
func (p AMap) InContext(ctx Ctx) Ctx {
	return context.WithValue(ctx, noop{}, p)
}

// Return an AMap with the keys/values from the passed-in AMap added to and/or
// replacing the keys/values from the method receiver.
func (a AMap) Merge(b AMap) AMap {
	m := 0
	if nil != a {
		m = len(a.keys)
	}
	if 0 == m {
		return b
	}

	n := 0
	if nil != b {
		n = len(b.keys)
	}
	if 0 == n {
		return a
	}

	keys := make([]string, m+n)
	vals := make([]interface{}, m+n)
	idx := make(map[string]int, m+n)
	copy(keys, a.keys)
	copy(vals, a.vals)
	for i, k := range a.keys {
		idx[k] = i
	}

	o := m
	for i, key := range b.keys {
		val := b.vals[i]
		if j, ok := idx[key]; ok {
			vals[j] = val
		} else {
			keys[o] = key
			vals[o] = val
			idx[key] = o
			o++
		}
	}
	return &KVPairs{keys: keys[:o], vals: vals[:o]}
}

// Return an AMap with the passed-in key/value pairs added to and/or replacing
// the keys/values from the method receiver.
func (p AMap) AddPairs(pairs ...interface{}) AMap {
	n := len(pairs)
	if 0 == n {
		return p
	}
	n = (n + 1) / 2

	m := 0
	if nil != p {
		m = len(p.keys)
	}

	keys := make([]string, m+n)
	vals := make([]interface{}, m+n)
	idx := make(map[string]int, m+n)
	if nil != p {
		copy(keys, p.keys)
		copy(vals, p.vals)
		for i, k := range p.keys {
			idx[k] = i
		}
	}
	o := m
	for i := 0; i < n; i++ {
		key := S(pairs[2*i])
		val := interface{}(nil)
		if 2*i+1 < len(pairs) {
			val = pairs[2*i+1]
		}
		if j, ok := idx[key]; ok {
			vals[j] = val
		} else {
			keys[o] = key
			vals[o] = val
			idx[key] = o
			o++
		}
	}
	return &KVPairs{keys: keys[:o], vals: vals[:o]}
}
