# tcpserver

*tcpserver* is an extremely fast and flexible IPv4 and IPv6 capable TCP server with TLS support, graceful shutdown and some TCP tuning options like `TCP_FASTOPEN`, `SO_RESUSEPORT` and `TCP_DEFER_ACCEPT`.

This library requires at least Go 1.11 but has no other dependencies, does not contain ugly and incompatible hacks and thus fully integrates into Go's `net/*` universe.


## Architecture
*tcpserver* uses a multiple accept-loop approach while spawning a new go routine (`RequestHandlerFunc func(conn *Connection)`) for each connection.
The connection is automatically closed when the request handler function returns.

Memory allocation in hot paths are reduced to a minimum using golang's `sync.Pool`.

A small performance increase is still possible by using a go routine pool instead of spawning a new go routine for each connection.


## Example

```golang
server, err := tcpserver.NewServer("127.0.0.1:5000")

server.SetRequestHandler(requestHandler)
server.Listen()
server.Serve()
```


## Benchmarks

Benchmarks are always tricky, especially those that depend on network operations. I've tried my best to get fair and realistic results.
 
The test server, a dual `Intel(R) Xeon(R) CPU E5-2620 v4 @ 2.10GHz` with a total of 32 cores and 128 GB of memory, was installed from scratch with latest Debian 10.1 (Buster) and Linux Kernel 4.19.67-2. There were no other network daemons running except SSH. 

[Bombardier](https://github.com/codesenberg/bombardier) (latest commit [9a0fa99](https://github.com/codesenberg/bombardier/tree/9a0fa99d0334574700f31150c9d72a3eefc05092) from 2019-02-22) was used as HTTP benchmarking tool.

All tests were performed against localhost.

 


### Echo server

### Simple HTTP server


## License

*tcpserver* is available under the MIT [license](LICENSE).
