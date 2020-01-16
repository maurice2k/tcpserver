// Copyright 2017 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/tidwall/evio"
)

var res string
var resbytes []byte

type request struct {
	proto, method string
	path, query   string
	head, body    string
	remoteAddr    string
}

var listenAddr string
var keepAlive bool
var sleep int
var aes128 bool
var sha bool

var status200Ok = []byte("200 OK")
var status500Error = []byte("500 Error")

var aesKey = []byte("0123456789ABCDEF")

func main() {
	/*go func() {
		defer os.Exit(0)
		cpuProfile, err := os.Create("evio-cpu.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
		pprof.StartCPUProfile(cpuProfile)

		time.Sleep(time.Second * 10)
		fmt.Println("Writing cpu & mem profile...")

		// Memory Profile
		memProfile, err := os.Create("evio-mem.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer memProfile.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(memProfile); err != nil {
			log.Fatal(err)
		}
	}()/**/

	var loops int
	var aaaa int
	var unixsocket string
	var stdlib bool

	flag.StringVar(&unixsocket, "unixsocket", "", "unix socket")
	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.IntVar(&aaaa, "aaaa", 0, "aaaaa.... (default output is 'Hello World')")
	flag.BoolVar(&stdlib, "stdlib", false, "use stdlib")
	flag.IntVar(&loops, "loops", 0, "num loops")
	flag.BoolVar(&keepAlive, "keepalive", true, "use HTTP Keep-Alive")
	flag.BoolVar(&aes128, "aes128", false, "encrypt response with aes-128-cbc")
	flag.BoolVar(&sha, "sha", false, "output sha256 instead of plain response")
	flag.IntVar(&sleep, "sleep", 0, "sleep number of milliseconds per request")
	flag.Parse()

	if aaaa > 0 {
		res = strings.Repeat("a", aaaa)
	} else {
		res = "Hello World!\r\n"
	}

	resbytes = []byte(res)

	var events evio.Events
	events.NumLoops = loops
	events.Serving = func(srv evio.Server) (action evio.Action) {
		log.Printf("http server using evio started on %s with GOMAXPROCS=%d (loops: %d)", listenAddr, runtime.GOMAXPROCS(0), srv.NumLoops)
		if unixsocket != "" {
			log.Printf("http server started at %s", unixsocket)
		}
		if stdlib {
			log.Printf("stdlib")
		}
		return
	}

	events.Opened = func(c evio.Conn) (out []byte, opts evio.Options, action evio.Action) {
		c.SetContext(&evio.InputStream{})
		//log.Printf("opened: laddr: %v: raddr: %v", c.LocalAddr(), c.RemoteAddr())
		return
	}

	events.Closed = func(c evio.Conn, err error) (action evio.Action) {
		//log.Printf("closed: %s: %s", c.LocalAddr().String(), c.RemoteAddr().String())
		return
	}

	events.Data = func(c evio.Conn, in []byte) (out []byte, action evio.Action) {
		if in == nil {
			return
		}
		is := c.Context().(*evio.InputStream)
		data := is.Begin(in)
		// process the pipeline
		var req request
		leftover, err := parsereq(data, &req)
		if err != nil {
			// bad thing happened
			out = appendresp(out, status500Error, nil, []byte(err.Error()+"\n"))
			action = evio.Close
			return
		} else if len(leftover) == len(data) {
			// request not ready, yet
			return
		}
		// handle the request
		if aes128 {
			cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
			out = appendresp(out, status200Ok, nil, cryptedResbytes)
		} else if sha {
			sha256sum := sha256.Sum256(resbytes)
			out = appendresp(out, status200Ok, nil, []byte(hex.EncodeToString(sha256sum[:])))
		} else {
			out = appendresp(out, status200Ok, nil, resbytes)
		}

		data = leftover

		if sleep > 0 {
			time.Sleep(time.Millisecond * time.Duration(sleep))
		}

		if !keepAlive {
			action = evio.Close
		}
		is.End(data)
		return
	}
	var ssuf string
	if stdlib {
		ssuf = "-net"
	}
	// We at least want the single http address.
	addrs := []string{fmt.Sprintf("tcp"+ssuf+"://%s", listenAddr)}
	if unixsocket != "" {
		addrs = append(addrs, fmt.Sprintf("unix"+ssuf+"://%s", unixsocket))
	}
	// Start serving!
	log.Fatal(evio.Serve(events, addrs...))
}

var headerHTTP11 = []byte("HTTP/1.1")
var headerDate = []byte("Date: ")
var headerConnectionClose = []byte("Connection: close")
var headerServerIdentity = []byte("Server: evio")
var headerContentLength = []byte("Content-Length: ")
var headerContentType = []byte("Content-Type: ")
var headerContentTypeTextPlain = []byte("text/plain")
var newLine = []byte("\r\n")

// appendresp will append a valid http response to the provide bytes.
// The status param should be the code plus text such as "200 OK".
// The head parameter should be a series of lines ending with "\r\n" or empty.
func appendresp(b []byte, status, head, body []byte) []byte {
	b = append(b, headerHTTP11...)
	b = append(b, ' ')
	b = append(b, status...)
	b = append(b, newLine...)
	b = append(b, headerServerIdentity...)
	b = append(b, newLine...)
	if !keepAlive {
		b = append(b, headerConnectionClose...)
		b = append(b, newLine...)
	}
	b = append(b, headerDate...)
	b = time.Now().AppendFormat(b, "Mon, 02 Jan 2006 15:04:05 GMT")
	b = append(b, newLine...)
	if len(body) > 0 {
		b = append(b, headerContentType...)
		b = append(b, headerContentTypeTextPlain...)
		b = append(b, newLine...)
		b = append(b, headerContentLength...)
		b = strconv.AppendInt(b, int64(len(body)), 10)
		b = append(b, newLine...)
	}
	b = append(b, head...)
	b = append(b, newLine...)
	if len(body) > 0 {
		b = append(b, body...)
	}
	return b
}

// parsereq is a very simple http request parser. This operation
// waits for the entire payload to be buffered before returning a
// valid request.
func parsereq(data []byte, req *request) (leftover []byte, err error) {
	sdata := data
	var i, s int
	var top string
	var clen int
	var q = -1
	// method, path, proto line
	for ; i < len(sdata); i++ {
		if sdata[i] == ' ' {
			req.method = b2s(sdata[s:i])
			for i, s = i+1, i+1; i < len(sdata); i++ {
				if sdata[i] == '?' && q == -1 {
					q = i - s
				} else if sdata[i] == ' ' {
					if q != -1 {
						req.path = b2s(sdata[s:q])
						req.query = req.path[q+1 : i]
					} else {
						req.path = b2s(sdata[s:i])
					}
					for i, s = i+1, i+1; i < len(sdata); i++ {
						if sdata[i] == '\n' && sdata[i-1] == '\r' {
							req.proto = b2s(sdata[s:i])
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
	top = b2s(sdata[:s])
	for ; i < len(sdata); i++ {
		if i > 1 && sdata[i] == '\n' && sdata[i-1] == '\r' {
			line := b2s(sdata[s : i-1])
			s = i + 1
			if line == "" {
				req.head = b2s(sdata[len(top)+2 : i+1])
				i++
				if clen > 0 {
					if len(sdata[i:]) < clen {
						break
					}
					req.body = b2s(sdata[i : i+clen])
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

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// Encrypts given cipher text (prepended with the IV) with AES-128 or AES-256
// (depending on the length of the key)
func encryptCBC(plainText, key []byte) (cipherText []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plainText = pad(aes.BlockSize, plainText)

	cipherText = make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], plainText)

	return cipherText, nil
}

// Adds PKCS#7 padding (variable block length <= 255 bytes)
func pad(blockSize int, buf []byte) []byte {
	padLen := blockSize - (len(buf) % blockSize)
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(buf, padding...)
}
