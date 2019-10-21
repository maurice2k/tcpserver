// Copyright 2017 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/tidwall/evio"
)

func main() {
	var listenAddr string
	var loops int
	var udp bool
	var trace bool
	var reuseport bool
	var stdlib bool

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.BoolVar(&udp, "udp", false, "listen on udp")
	flag.BoolVar(&reuseport, "reuseport", false, "reuseport (SO_REUSEPORT)")
	flag.BoolVar(&trace, "trace", false, "print packets to console")
	flag.IntVar(&loops, "loops", 0, "num loops")
	flag.BoolVar(&stdlib, "stdlib", false, "use stdlib")
	flag.Parse()

	var events evio.Events
	events.NumLoops = loops
	events.Serving = func(srv evio.Server) (action evio.Action) {
		log.Printf("echo server started on %s with GOMAXPROCS=%d (loops: %d)", listenAddr, runtime.GOMAXPROCS(0), srv.NumLoops)
		if reuseport {
			log.Printf("reuseport")
		}
		if stdlib {
			log.Printf("stdlib")
		}
		return
	}
	events.Data = func(c evio.Conn, in []byte) (out []byte, action evio.Action) {
		if trace {
			log.Printf("%s", strings.TrimSpace(string(in)))
		}
		out = in
		return
	}
	scheme := "tcp"
	if udp {
		scheme = "udp"
	}
	if stdlib {
		scheme += "-net"
	}
	log.Fatal(evio.Serve(events, fmt.Sprintf("%s://%s?reuseport=%t", scheme, listenAddr, reuseport)))
}
