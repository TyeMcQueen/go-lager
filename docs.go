/*
Lager makes logs that are easy for computers to parse, easy for people to
read, and easy for programmers to generate.  It also encourages logging data
over messages, which tends to make logs more useful as well as easier to
generate.

You don't need to pass around a logging object so you can log information
from any code.  You can decorate a Go context.Context with additional data
to be added to each log line written when that context applies.

The logs are written in JSON format but the items in JSON are written in
a controlled order, preserving the order used in the program code.  This
makes the logs pretty easy for humans to scan even with no processing or
tooling.

Typical logging code like:

	lager.Fail(ctx).MMap("Can't merge", "dest", dest, "err", err)
	// MMap() takes a message followed by a map of key/value pairs.

could output (especially when running interactively):

	["2019-12-31 23:59:59.1234Z", "FAIL", "Can't merge",
		{"dest":"localhost", "err":"refused"}]

(but as a single line).  If you declare that the code is running inside
Google Cloud Platform (GCP), it could instead output:

	{"time":"2019-12-31T23:59:59.1234Z", "severity":"500",
		"message":"Can't merge", "dest":"localhost", "err":"refused"}

(as a single line) which GCP understands well but note that it is still
easy for a human to read with the consistent order used.

You don't even need to take the time to compose and type labels for data
items, if it doesn't seem worth it in some cases:

	lager.Fail(ctx).MList("Can't merge", dest, err)
	// MList() takes a message followed by arbitrary data items.

There are 11 log levels and 9 can be independently enabled or disabled.  You
usually use them via code similar to:

	lager.Panic().MMap(...) // Calls panic(), always enabled.
	lager.Exit().MMap(...)  // To Stderr, calls os.Exit(1), always enabled.
	lager.Fail().MMap(...)  // For non-normal failures (default on).
	lager.Warn().MMap(...)  // For warnings (default on).
	lager.Note().MMap(...)  // For normal, important notices (default on).
	lager.Acc().MMap(...)   // For access logs (default on).
	lager.Info().MMap(...)  // For normal events (default off).
	laber.Trace().MMap(...) // For tracing code flow (default off).
	laber.Debug().MMap(...) // For debugging details (default off).
	laber.Obj().MMap(....)  // For object dumps (default off).
	lager.Guts().MMap(...)  // For volumious data dumps (default off).

Panic and Exit cannot be disabled.  Fail, Warn, Note, and Acc are enabled by
default.

If you want to decorate each log line with additional key/value pairs, then
you can accumulate those in a context.Context value that gets passed around
and then pass that Context in when logging:

	// In a method dispatcher, for example:
	ctx = lager.AddPairs(ctx, "srcIP", ip, "user", username)

	// In a method or code that a method calls, for example:
	lager.Info(ctx).MMap(msg, pairs...)

	// Or, if you have multiple log lines at the same level:
	debug := lager.Debug(ctx)
	debug.MMap(msg, pairs...)
	debug.MList(msg, args...)

Most log archiving systems expect JSON log lines to be a map (object/hash)
not a list (array).  To get that you just declare what labels to use for:
timestamp, level, message, list data, context, and module.

	// Example choice of logging keys:
	lager.Keys("t", "l", "msg", "a", "", "mod")

Support for GCP Cloud Logging and Cloud Trace is integrated.

*/
package lager
