Lager
==

Logs (in golang) that are easy for computers to parse, easy for people to
read, and easy for programmers to generate.  The logs are written in JSON
format but the order of the data in each line is controlled to make the
logs quite nice for humans to read even with no processing or tooling.

This is most similar to https://github.com/uber-go/zap and is about as
efficient in CPU usage and lack of allocations per log line.  But this
efficiency was not allowed to make it inconvenient to write code that uses
this library.

Another departure from most logging systems is that Lager encourages you to
log data, not messages.  This difference can be a little subtle, but it can
make it faster to add logging to code while also making it more likely that
you'll be able to parse out the data you care about from the log rather than
having to do string matching.

It also provides more granularity than is typical when controlling which log
lines to write and which to suppress.  It offers 9 log levels, most of which
can be enabled individually (enabling "Debug" does not force you to also
enable "Info").  You can also easily allow separate log levels for specific
packages or any other logical division you care to use.

Typical logging code:

    lager.Fail(ctx).Map("Err", err, "for", obj)

might produce a log line like:

    ["2018-12-31 23:59:59.870Z", "FAIL", {"Err":"bad cost", "for":{"cost":-1}}]

Note that the key/value pairs are always logged in the same order you listed
them in your code.

But you can even avoid having to come up with key/value pairs, which
can be convenient for quickly adding logging:

    lager.Debug(ctx).List("Connecting to", host, port, path)

In order to be efficient and to keep control over the order of output,
Lager mostly avoids using the "encoding/json" library and also does not use
reflection.  But you can pass in any arbitrary data.  If it isn't one of the
many data types natively supported by Lager, then it will use "encoding/json"
to marshal it.
