package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"tcpserver"
)

var port int
var keepAlive bool

func main() {
	tfMap := make(map[bool]string)
	tfMap[true] = "on"
	tfMap[false] = "off"

	flag.IntVar(&port, "port", 8000, "server port")
	flag.BoolVar(&keepAlive, "keepalive", true, "use HTTP Keep-Alive")
	flag.Parse()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

	fmt.Printf("Running http server on %s\n", listenAddr)
	fmt.Printf(" - keepalive: %s\n", tfMap[keepAlive])

	server, _ := tcpserver.NewServer(listenAddr)
	server.SetListenConfig(&tcpserver.ListenConfig{
		SocketReusePort: true,
		SocketFastOpen:    false,
		SocketDeferAccept: false,
	})
	server.SetRequestHandler(requestHandler)
	err := server.Listen()
	if err != nil {
		panic("Error listening on interface: " + err.Error())
	}

	err = server.Serve()
	if err != nil {
		panic("Error serving: " + err.Error())
	}
}

type request struct {
	proto, method string
	path, query   string
	head, body    string
	remoteAddr    string
}

func requestHandler(conn *tcpserver.Connection) {
	buf := make([]byte, 1024)
	var data, out []byte
	var req request
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		data = append(data, buf[0:n]...)
		leftover, err := parsereq(data, &req)
		if err != nil {
			// bad thing happened
			out = appendresp(out, "500 Error", "", err.Error()+"\n")
			break
		} else if len(leftover) == len(data) {
			// request not ready, yet
			continue
		}
		// handle the request
		req.remoteAddr = conn.RemoteAddr().String()

		sha1Str := []byte("This page intentionally left blank.")
		sha1Str[0] = byte(rand.Int())
		sha1Str[1] = byte(rand.Int())
		sha1Bytes := sha1.Sum(sha1Str)

		out = appendresp(out, "200 OK", "", hex.EncodeToString(sha1Bytes[:]))
		time.Sleep(time.Millisecond * 1)
		conn.Write(out)

		data = nil
		out = nil
	}

	return
}

var res string = "Hello World!\r\n"

// appendhandle handles the incoming request and appends the response to
// the provided bytes, which is then returned to the caller.
func appendhandle(b []byte, req *request) []byte {
	return appendresp(b, "200 OK", "", res)
}

// appendresp will append a valid http response to the provide bytes.
// The status param should be the code plus text such as "200 OK".
// The head parameter should be a series of lines ending with "\r\n" or empty.
func appendresp(b []byte, status, head, body string) []byte {
	b = append(b, "HTTP/1.1"...)
	b = append(b, ' ')
	b = append(b, status...)
	b = append(b, '\r', '\n')
	b = append(b, "Server: tsrv\r\n"...)
	//b = append(b, "Connection: close\r\n"...)
	b = append(b, "Date: "...)
	b = time.Now().AppendFormat(b, "Mon, 02 Jan 2006 15:04:05 GMT")
	b = append(b, '\r', '\n')
	if len(body) > 0 {
		b = append(b, "Content-Length: "...)
		b = strconv.AppendInt(b, int64(len(body)), 10)
		b = append(b, '\r', '\n')
	}
	b = append(b, head...)
	b = append(b, '\r', '\n')
	if len(body) > 0 {
		b = append(b, body...)
	}
	return b
}

// parsereq is a very simple http request parser. This operation
// waits for the entire payload to be buffered before returning a
// valid request.
func parsereq(data []byte, req *request) (leftover []byte, err error) {
	sdata := string(data)
	var i, s int
	var top string
	var clen int
	var q = -1
	// method, path, proto line
	for ; i < len(sdata); i++ {
		if sdata[i] == ' ' {
			req.method = sdata[s:i]
			for i, s = i+1, i+1; i < len(sdata); i++ {
				if sdata[i] == '?' && q == -1 {
					q = i - s
				} else if sdata[i] == ' ' {
					if q != -1 {
						req.path = sdata[s:q]
						req.query = req.path[q+1 : i]
					} else {
						req.path = sdata[s:i]
					}
					for i, s = i+1, i+1; i < len(sdata); i++ {
						if sdata[i] == '\n' && sdata[i-1] == '\r' {
							req.proto = sdata[s:i]
							i, s = i+1, i+1
							break
						}
					}
					break
				}
			}
			break
		}
	}
	if req.proto == "" {
		return data, fmt.Errorf("malformed request")
	}
	top = sdata[:s]
	for ; i < len(sdata); i++ {
		if i > 1 && sdata[i] == '\n' && sdata[i-1] == '\r' {
			line := sdata[s : i-1]
			s = i + 1
			if line == "" {
				req.head = sdata[len(top)+2 : i+1]
				i++
				if clen > 0 {
					if len(sdata[i:]) < clen {
						break
					}
					req.body = sdata[i : i+clen]
					i += clen
				}
				return data[i:], nil
			}
			if strings.HasPrefix(line, "Content-Length:") {
				n, err := strconv.ParseInt(strings.TrimSpace(line[len("Content-Length:"):]), 10, 64)
				if err == nil {
					clen = int(n)
				}
			}
		}
	}
	// not enough data
	return data, nil
}
