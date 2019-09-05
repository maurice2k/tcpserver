package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func main() {
	var res string
	var listenAddr string
	var aaaa int
	var keepAlive bool
	var sha bool
	var sleep int

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.IntVar(&aaaa, "aaaa", 0, "aaaaa.... (default output is 'Hello World')")
	flag.BoolVar(&keepAlive, "keepalive", true, "use HTTP Keep-Alive")
	flag.BoolVar(&sha, "sha", false, "output sha256 instead of plain response")
	flag.IntVar(&sleep, "sleep", 0, "sleep number of milliseconds per request")
	flag.Parse()

	if aaaa > 0 {
		res = strings.Repeat("a", aaaa)
	} else {
		res = "Hello World!\r\n"
	}

	go log.Printf("http server started on %s", listenAddr)
	s := &fasthttp.Server{
		Handler: func(c *fasthttp.RequestCtx) {

			if sha {
				sha256sum := sha256.Sum256([]byte(res))
				c.WriteString(hex.EncodeToString(sha256sum[:]))
			} else {
				c.WriteString(res)
			}

			if sleep > 0 {
				time.Sleep(time.Millisecond * time.Duration(sleep))
			}
		},
		DisableKeepalive: !keepAlive,
	}
	s.ListenAndServe(listenAddr)
}
