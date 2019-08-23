package main

import (
	"flag"
	"fmt"
	"io"
	"net"

	"tcpserver"
)

var port int
var zeroCopy bool

func main() {
	tfMap := make(map[bool]string)
	tfMap[true] = "on"
	tfMap[false] = "off"

	flag.IntVar(&port, "port", 5001, "server port")
	flag.BoolVar(&zeroCopy, "zerocopy", true, "use splice/sendfile zero copy")
	flag.Parse()

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

	fmt.Printf("Running echo server on %s\n", listenAddr)
	fmt.Printf(" - zerocopy: %s\n", tfMap[zeroCopy])

	server, _ := tcpserver.NewServer(listenAddr)
	server.SetListenConfig(&tcpserver.ListenConfig{
		SocketReusePort:   true,
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

func requestHandler(conn *tcpserver.Connection) {
	if zeroCopy {
		_, _ = io.Copy(conn.Conn.(*net.TCPConn), conn.Conn.(*net.TCPConn))
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
