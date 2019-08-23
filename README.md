# tcpserver

*tcpserver* is a fast and flexible IPv4 and IPv6 capable TCP server with TLS support, graceful shutdown and some TCP tuning options like `TCP_FASTOPEN`, `SO_RESUSEPORT` and `TCP_DEFER_ACCEPT`.

This library requires at least Go 1.11 but has no other dependencies, does not contain ugly and incompatible hacks and thus  fully integrates into Go's `net/*` universe.


## Architecture
*tcpserver* uses the classic blocking accept-loop approach while spawning a new go routine (`RequestHandlerFunc func(conn *Connection)`) for each connection.
The connection is automatically closed when the request handler function returns.

Using a worker pool instead of spawning a new go routine has been tested for speed but with no significant results given the downsides of this approach (i.e. fixed pool size blocks if concurrency goes beyond the pool size).

## Benchmarks

### Echo erver

### Simple HTTP server


## License

*tcpserver* is available under the MIT [license](LICENSE).