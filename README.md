# tcpserver

*tcpserver* is an extremely fast and flexible IPv4 and IPv6 capable TCP server with TLS support, graceful shutdown and some TCP tuning options like `TCP_FASTOPEN`, `SO_RESUSEPORT` and `TCP_DEFER_ACCEPT`.

This library requires at least Go 1.11 but has no other dependencies, does not contain ugly and incompatible hacks and thus  fully integrates into Go's `net/*` universe.


## Architecture
*tcpserver* uses a multiple accept-loop approach while spawning a new go routine (`RequestHandlerFunc func(conn *Connection)`) for each connection.
The connection is automatically closed when the request handler function returns.

Memory allocation in hot paths are reduced to a minimum using golang's `sync.Pool`.

A small performance increase is still possible by using a go routine pool instead of spawning a new go routine for each connection. 


## Benchmarks

Never ever trust benchmarks. 


### Echo server

### Simple HTTP server


## License

*tcpserver* is available under the MIT [license](LICENSE).
