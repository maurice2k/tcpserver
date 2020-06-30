package main

import (
	"github.com/maurice2k/tcpserver"

	"crypto/tls"
	"flag"
	"fmt"
	"io"
)

var listenAddr string
var port int
var zeroCopy bool
var secure bool

func main() {
	tfMap := make(map[bool]string)
	tfMap[true] = "on"
	tfMap[false] = "off"

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:5000", "server listen addr")
	flag.BoolVar(&zeroCopy, "zerocopy", true, "use splice/sendfile zero copy")
	flag.BoolVar(&secure, "secure", false, "use TLS")
	flag.Parse()

	fmt.Printf("Running echo server on %s\n", listenAddr)
	fmt.Printf(" - zerocopy: %s\n", tfMap[zeroCopy])
	fmt.Printf(" - TLS secured: %s\n", tfMap[secure])

	server, _ := tcpserver.NewServer(listenAddr)

	if secure {
		cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
		if err != nil {
			panic("Error loading servert cert and key file: " + err.Error())
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		server.SetTLSConfig(tlsConfig)
	}

	server.SetListenConfig(&tcpserver.ListenConfig{
		SocketReusePort:   true,
		SocketFastOpen:    false,
		SocketDeferAccept: false,
	})
	server.SetRequestHandler(requestHandler)
	var err error
	if secure {
		err = server.ListenTLS()
	} else {
		err = server.Listen()
	}

	if err != nil {
		panic("Error listening on interface: " + err.Error())
	}

	err = server.Serve()
	if err != nil {
		panic("Error serving: " + err.Error())
	}
}

func requestHandler(conn *tcpserver.TCPConn) {
	if zeroCopy {
		// automatically uses zero copy if conn.Conn is of type net.TCPConn,
		// otherwise does a normal user space copy
		_, _ = io.Copy(conn.Conn, conn.Conn)
	} else {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				break
			}

			_, _ = conn.Write(buf[:n])
		}
	}
}
