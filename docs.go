/*
Logs that are easy for computers to parse, easy for people to read, and easy
for programmers to generate.  The logs are written in JSON format but the
order of the data in each line is controlled to make the logs quite nice for
humans to read even with no processing or tooling.

	lager.Fail(ctx).List("Can't reach", dest, err)

could output:

	["2018-12-31 23:59:59.860Z", "FAIL", "Can't reach", "localhost", "refused"]

unless the Fail log level had been disabled.  While:

	lager.Info().Map("Err", err, "for", obj)

logs nothing by default.  But if Info log level is enabled, it might log:

	["2018-12-31 23:59:59.870Z", "INFO", {"Err":"bad cost", "for":{"cost":-1}}]

There are 9 log levels and 7 can be separately enabled or diabled.  You
usually use them via code similar to:

	lager.Panic().List(args...) // Calls panic(), always enabled.
	lager.Exit().Map(pairs...)  // To Stderr, calls os.Exit(1), always enabled.
	lager.Fail().List(args...)  // For non-normal failures (default on).
	lager.Warn().Map(pairs...)  // For warnings (default on).
	lager.Acc().Map(pairs...)   // For access logs (default on).
	lager.Info().List(args...)  // For normal events (default off).
	laber.Trace().List(args...) // For tracing code flow (default off).
	laber.Debug().Map(pairs...) // For debugging details (default off).
	laber.Obj().List(args....)  // For object dumps (default off).
	lager.Guts().Map(pairs...)  // For volumious data dumps (default off).

The Panic and Exit levels cannot be disabled.  Of the others, only Fail, Warn,
and Acc levels are enabled by default.  When the Lager module is initialized,
the following code is called:

	lager.Init(os.Getenv("LAGER_LEVELS"))

and lager.Init("") is the same as lager.Init("Fail Warn Acc") [which is the
same as lager.Init("FWA")].  So you can set the LAGER_LEVELS environment
variable to change which log levels are enabled.  Application code can also
choose to support other ways to override those defaults.

If you want to decorate each log line with additional key/value pairs, then
you can accumulate those in a context.Context value that gets passed around
and then pass that Context in when logging:

	// In a method dispatcher, for example:
	ctx = lager.AddPairs(ctx, "sourceIP", ip, "user", username)

	// In a method or code that a method calls, for example:
	lager.Info(ctx).Map(pairs...)

	// Or, if you have multiple log lines at the same level:
	debug := lager.Debug(ctx)
	debug.Map(pairs...)
	debug.List(args...)

*/
package lager
