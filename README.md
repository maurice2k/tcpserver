# tcpserver

*tcpserver* is an extremely fast and flexible **IPv4 and IPv6** capable TCP server with **TLS support**, graceful shutdown, **Zero-Copy** (on Linux with splice/sendfile) and supports TCP tuning options like `TCP_FASTOPEN`, `SO_RESUSEPORT` and `TCP_DEFER_ACCEPT`.

This library requires at least Go 1.11 but has no other dependencies, does not contain ugly and incompatible hacks and thus fully integrates into Go's `net/*` universe.


## Architecture
*tcpserver* uses a multi-accept-loop approach while spawning a new go routine (`RequestHandlerFunc func(conn *Connection)`) for each connection.
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

The test server, a dual `Intel(R) Xeon(R) CPU E5-2620 v4 @ 2.10GHz` machine with a total of 32 cores and 128 GB of memory, was installed from scratch with latest Debian 10.1 (Buster) and Linux Kernel 4.19.67-2. There were no other network daemons running except for SSH. 

The tests are performed using a super simple HTTP server implemention on top of tcpserver and the other tested libraries. HTTP is well known, there are good benchmarking tools and it's easy to test throughput (using HTTP Keep-Alive) as well as handling thousands of short-lived connections.  

[Bombardier](https://github.com/codesenberg/bombardier) (latest commit [9a0fa99](https://github.com/codesenberg/bombardier/tree/9a0fa99d0334574700f31150c9d72a3eefc05092) from 2019-02-22) was used as HTTP benchmarking tool.

The following libraries were benchmarked:
- [evio](https://github.com/tidwall/evio)
- [gnet](https://github.com/panjf2000/gnet)
- [tcpserver](https://github.com/maurice2k/tcpserver)
- [fasthttp](https://github.com/valyala/fasthttp) (just to see a good HTTP library, also based on `net/*` like tcpserver)
- [net/http](https://golang.org/pkg/net/http/) (Golang's own HTTP server implementation)

All tests were performed against localhost.

**Hint**: If you're looking for a high performant HTTP library, just use [fasthttp](https://github.com/valyala/fasthttp). It is extremly fast and has good HTTP support. The second best option is probably to stick to [net/http](https://golang.org/pkg/net/http/). evio, gnet and tcpserver are primarily designed for other use cases like your own protocols, proxy servers and the like. Don't re-invent the wheel ;)

## Test #1: Static 1kB content throughput test
100 concurrent clients, 1kB of HTTP payload returned, Keep-Alive turned on, 10 seconds (establishes exactly 100 TCP connections that are serving HTTP requests).

## Test #2: Static 1kB content massive connections test
100 concurrent clients, 1kB of HTTP payload returned, Keep-Alive turned off, 10 seconds (each HTTP request is a new connection).

## Test #3: 



## License

*tcpserver* is available under the MIT [license](LICENSE).
