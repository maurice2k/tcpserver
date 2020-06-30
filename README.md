# tcpserver

*tcpserver* is an extremely fast and flexible **IPv4 and IPv6** capable TCP server with **TLS support**, graceful shutdown, **Zero-Copy** (on Linux with splice/sendfile) and supports TCP tuning options like `TCP_FASTOPEN`, `SO_RESUSEPORT` and `TCP_DEFER_ACCEPT`.

This library requires at least Go 1.11 but has no other dependencies, does not contain ugly and incompatible hacks and thus fully integrates into Go's `net/*` universe.


## Architecture
*tcpserver* uses a multi-accept-loop approach combined with an adaptive spawning go routine pool for incoming connections.

Each connection lives in it's own go routine and it served using a request handler function (`RequestHandlerFunc func(conn *Connection)`). 
The connection is automatically closed when the request handler function returns.

Memory allocations in hot paths are reduced to a minimum using `sync.Pool` and the go routine pool from `maurice2k/ultrapool`

As *tcpserver* does not implement a non-blocking/asynchronous event loop itself (like packages such as *evio* or *gnet*) it is fully compatible with everything that is built on top of `net.TCPConn`. 


## Example (echo server)

```golang
server, err := tcpserver.NewServer("127.0.0.1:5000")

server.SetRequestHandler(requestHandler)
server.Listen()
server.Serve()

func requestHandler(conn *tcpserver.TCPConn) {
    io.Copy(conn.Conn.(*net.TCPConn), conn.Conn.(*net.TCPConn))
}
```


## Benchmarks

Benchmarks are always tricky, especially those that depend on network operations. I've tried my best to get fair and realistic results (given that these benchmarks are of course very synthetic).

The test server, an *AWS c5.metal* dual `Intel(R) Xeon(R) Platinum 8275CL CPU @ 3.00GHz` machine with a total of 96 cores and 192 GB of memory, was installed with latest Amazon Linux 2, Linux Kernel 4.14.165-131.185.amzn2.x86_64 and Golang 1.14. There were no other network daemons running except for SSH.


The tests are performed using a super simple HTTP server implementation on top of tcpserver and the other tested libraries. HTTP is well known, there are good benchmarking tools and it's easy to test throughput (using HTTP Keep-Alive) as well as handling thousands of short-lived connections.  

[Bombardier](https://github.com/codesenberg/bombardier) (latest commit [9a0fa99](https://github.com/codesenberg/bombardier/tree/9a0fa99d0334574700f31150c9d72a3eefc05092) from 2019-02-22) was used as HTTP benchmarking tool.

The following libraries were benchmarked:
- [evio](https://github.com/tidwall/evio)
- [gnet](https://github.com/panjf2000/gnet)
- [tcpserver](https://github.com/maurice2k/tcpserver)
- [fasthttp](https://github.com/valyala/fasthttp) (just to see a good HTTP library, also based on `net/*` like tcpserver)
- [net/http](https://golang.org/pkg/net/http/) (Go's own HTTP server implementation)

All tests were performed against localhost.

**Hint**: If you're looking for a high performant HTTP library, just use [fasthttp](https://github.com/valyala/fasthttp). It is extremly fast and has extraordinary good HTTP support. The second best option is probably to stick to [net/http](https://golang.org/pkg/net/http/). evio, gnet and tcpserver are primarily designed for other use cases like your own protocols, proxy servers and the like. Don't re-invent the wheel ;)

## Test #1: Static 1kB content massive connections test
1000 concurrent clients, 1kB of HTTP payload returned, Keep-Alive turned off, 10 seconds (each HTTP request is a new connection).
![Test 01](benchmark/_results/aws-c5-metal/test01-graph.png)

## Test #2: Static 1kB content throughput test
1000 concurrent clients, 1kB of HTTP payload returned, Keep-Alive turned on, 10 seconds (establishes exactly 1000 TCP connections that are serving HTTP requests).
![Test 02](benchmark/_results/aws-c5-metal/test02-graph.png)

## Test #3: AES-128-CBC crypted 1kB content massive connections test 
1000 concurrent clients, 1kB of AES-128-CBC crypted HTTP payload returned, Keep-Alive turned off, 10 seconds (each HTTP request is a new connection).
![Test 03](benchmark/_results/aws-c5-metal/test03-graph.png)

## Test #4: AES-128-CBC crypted 1kB content throughput test
1000 concurrent clients, 1kB of AES-128-CBC crypted HTTP payload returned, Keep-Alive turned on, 10 seconds (establishes exactly 1000 TCP connections that are serving HTTP requests).
![Test 04](benchmark/_results/aws-c5-metal/test04-graph.png)

## Test #5: Static 128 byte content throughput test with additional 1ms sleep 
1000 concurrent clients, 128 bytes of HTTP payload returned and 1 ms sleep, Keep-Alive turned on, 10 seconds (establishes exactly 1000 TCP connections that are serving HTTP requests).
![Test 05](benchmark/_results/aws-c5-metal/test05-graph.png)

## Test #6: Static 16kB content massive connections test
1000 concurrent clients, 16kB of HTTP payload returned, Keep-Alive turned off, 10 seconds (each HTTP request is a new connection).
![Test 06](benchmark/_results/aws-c5-metal/test06-graph.png)

## License

*tcpserver* is available under the MIT [license](LICENSE).
