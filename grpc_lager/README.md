Lager gRPC Middleware
==

Middlewares for [gRPC Go](https://github.com/grpc/grpc-go) based off of [grpc-ecosystem/go-grpc-middleware](https://github.com/grpc-ecosystem/go-grpc-middleware)

Note: This library currently does not support streams or stream interceptors

Usage example:

```go
import (
    "github.com/grpc-ecosystem/go-grpc-middleware"
    "github.com/TyeMcQueen/go-lager/grpc_lager"
)

myServer := grpc.NewServer(
    grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_lager.UnaryServerInterceptor(),
        grpc_lager.PayloadUnaryServerInterceptor(deciderFunction)
    )),
)
```
