# Lager

Logs (in golang) that are easy for computers to parse, easy for people to
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

(as a single line) which GCP understands well.  Note that it is still
easy for a human to read with the consistent order used.

Lager is efficient but not to the point of making it inconvenient to write
code that uses Lager.

It also provides more granularity when controlling which log lines to write
and which to suppress.  It offers 11 log levels, most of which can be enabled
individually (enabling "Trace" does not force you to also enable "Debug").
You can also easily allow separate log levels for specific packages or any
other logical division you care to use.

## Forks

If you use a fork of this repository and want to have changes you make
accepted upstream, you should use the fork by adding (to go.mod in modules
where you use the fork) a line like:

    replace github.com/Unity-Technologies/go-lager-internal => github.com/your/lager v1.2.3

And then use "github.com/Unity-Technologies/go-lager-internal" in 'import' statements and
in a 'require' directive in the go.mod.  (See "replace directive" in
https://go.dev/ref/mod.)
