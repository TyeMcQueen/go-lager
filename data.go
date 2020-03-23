package lager

import(
	"context"
	"fmt"
)


// Storage for an ordered list of key/value pairs (without duplicate keys).
type KVPairs struct {
	keys    []string
	vals    []interface{}
}

// A list type that we efficiently convert to JSON.
type AList = []interface{}

// A raw list of key/value pairs we can efficiently convert to JSON as a map.
type RawMap []interface{}

// A processed list of key/value pairs we can efficiently convert to JSON.
type AMap = *KVPairs


// Converts an arbitrary value to a string.  Very similar to
// fmt.Sprintf("%v",arg) but treats []byte values the same as strings
// rather then dumping them as a list of byte values in base 10.
func S(arg interface{}) string {
	switch v := arg.(type) {
	case string: return v
	case []byte: return string(v)
	}
	return fmt.Sprintf("%v", arg)
}

// Returns a slice (lager.AList) that can be passed as an argument to another
// List() or Map() call to construct nested data that can be quickly serialized
// to JSON.
//
// lager.Info().Map("User", user, "not in", lager.List(one, two, three))
func List(args ...interface{}) AList { return args }

// Returns a raw list of key/value pairs (lager.RawMap) that can be passed as
// an argument to another List() or Map() call to construct nested data that
// can be quickly serialized to JSON.
//
// Dupliate keys all get output.  If you need to squash duplicate keys, then
// call lager.Pairs() instead.
//
// lager.Info(ctx).List("Using", lager.Map("name", name, "age", age))
func Map(pairs ...interface{}) RawMap { return RawMap(pairs) }

// Returns a (processed) list of key/value pairs (lager.AMap) that can be
// added to a context.Context to specify additional data to append to each
// log line.
func Pairs(pairs ...interface{}) AMap {
	return AMap(nil).AddPairs(pairs...)
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
	n = (n+1)/2

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
