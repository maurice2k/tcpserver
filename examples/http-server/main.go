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
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/maurice2k/tcpserver"
)

var listenAddr string
var sleep int
var keepAlive bool
var aaaa int
var aes128 bool
var sha bool
var res string
var resbytes []byte

var status200Ok = []byte("200 OK")
var status500Error = []byte("500 Error")

var aesKey = []byte("0123456789ABCDEF")

func main() {
	tfMap := make(map[bool]string)
	tfMap[true] = "on"
	tfMap[false] = "off"

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.IntVar(&aaaa, "aaaa", 0, "aaaaa.... (default output is 'Hello World')")
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

	fmt.Printf("Running http server on %s with GOMAXPROCS=%d\n", listenAddr, runtime.GOMAXPROCS(0))
	fmt.Printf(" - keepalive: %s\n", tfMap[keepAlive])
	if sleep > 0 {
		fmt.Printf(" - sleep ms per request: %d ms\n", sleep)
	}
	if aes128 {
		fmt.Printf(" - encrypt response with aes-128-cbc\n")
	}
	if !aes128 && sha {
		fmt.Printf(" - output sha256 of reponse\n")
	}

	server, _ := tcpserver.NewServer(listenAddr)
	server.SetListenConfig(&tcpserver.ListenConfig{
		SocketReusePort:   false,
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

type reqVars struct {
	buf,
	out,
	data,
	leftover []byte
	req request
}

var reqVarsPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &reqVars{
			buf:  make([]byte, 2048),
			out:  make([]byte, 0, 2048),
			data: make([]byte, 0, 2048),
		}
	},
}

func requestHandler(conn *tcpserver.Connection) {
	rv := reqVarsPool.Get().(*reqVars)
	for {
		n, err := conn.Read(rv.buf[:2048])
		if err != nil {
			break
		}
		rv.data = append(rv.data, rv.buf[0:n]...)

		rv.leftover, err = parsereq(rv.data, &rv.req)
		if err != nil {
			// bad thing happened
			rv.out = appendresp(rv.out, status500Error, nil, []byte(err.Error()+"\n"))
			break
		}

		if len(rv.leftover) == len(rv.data) {
			// request not ready, yet
			continue
		}
		rv.out = rv.out[:0]
		// handle the request
		if aes128 {
			cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
			rv.out = appendresp(rv.out, status200Ok, nil, cryptedResbytes)
		} else if sha {
			sha256sum := sha256.Sum256(resbytes)
			rv.out = appendresp(rv.out, status200Ok, nil, []byte(hex.EncodeToString(sha256sum[:])))
		} else {
			rv.out = appendresp(rv.out, status200Ok, nil, resbytes)
		}

		if sleep > 0 {
			time.Sleep(time.Millisecond * time.Duration(sleep))
		}

		conn.Write(rv.out)

		if !keepAlive {
			break
		}

		rv.data = rv.data[0:0]
		rv.out = rv.out[0:0]
	}

	reqVarsPool.Put(rv)
	//*/
	return
}

var headerHTTP11 = []byte("HTTP/1.1")
var headerDate = []byte("Date: ")
var headerConnectionClose = []byte("Connection: close")
var headerServerIdentity = []byte("Server: tsrv")
var headerContentLength = []byte("Content-Length: ")
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

// s2b converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func s2b(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
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
