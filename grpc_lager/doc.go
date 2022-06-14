/*
grpc_lager is package for gRPC logging middlewares for the Lager library.
Based on middleware from https://github.com/grpc-ecosystem/go-grpc-middleware
The gRPC logging middleware populates request-scoped data to `grpc_ctxtags.Tags` that relate to the current gRPC call
(e.g. service and method names).
Once the gRPC logging middleware has added the gRPC specific Tags to the ctx they will then be written with the logs
that are made using the Lager logger.
All logging middleware will emit a final log statement. It is based on the error returned by the handler function,
the gRPC status code, an error (if any) and it will emit at a level controlled via `WithLevels`.

Field names
All field names of loggers follow the OpenTracing semantics definitions, with `grpc.` prefix if needed:
https://github.com/opentracing/specification/blob/master/semantic_conventions.md
*/
package grpc_lager
