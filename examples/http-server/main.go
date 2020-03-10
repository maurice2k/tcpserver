package main

import (
	"bufio"
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
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/maurice2k/tcpserver"
)

type request struct {
	proto, method string
	path, query   string
	head, body    string
	remoteAddr    string
}

type reqVars struct {
	data [2048]byte
	out []byte
	req request
}

var reqVarsPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return &reqVars{
			out: make([]byte, 0, 2048),
		}
	},
}

var serverDate atomic.Value
var bwPool *sync.Pool = &sync.Pool{}
var brPool *sync.Pool = &sync.Pool{}

var listenAddr string
var sleep int
var keepAlive bool
var aaaa int
var aes128 bool
var sha bool
var res string
var resbytes []byte
var loops int
var wpShards int

var status200Ok = []byte("200 OK")
var status500Error = []byte("500 Error")

var aesKey = []byte("0123456789ABCDEF")

func main() {
	/*go func() {
		defer os.Exit(0)
		cpuProfile, err := os.Create("tcpserver-cpu.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
		pprof.StartCPUProfile(cpuProfile)

		time.Sleep(time.Second * 10)
		fmt.Println("Writing cpu & mem profile...")

		// Memory Profile
		memProfile, err := os.Create("tcpserver-mem.prof")
		if err != nil {
			log.Fatal(err)
		}
		defer memProfile.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(memProfile); err != nil {
			log.Fatal(err)
		}
	}()/**/

	go func() {
		for {
			buf := appendTime(nil, time.Now())
			serverDate.Store(buf)
			time.Sleep(time.Second)
		}
	}()

	tfMap := make(map[bool]string)
	tfMap[true] = "on"
	tfMap[false] = "off"

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.IntVar(&aaaa, "aaaa", 0, "aaaaa.... (default output is 'Hello World')")
	flag.BoolVar(&keepAlive, "keepalive", true, "use HTTP Keep-Alive")
	flag.BoolVar(&aes128, "aes128", false, "encrypt response with aes-128-cbc")
	flag.BoolVar(&sha, "sha", false, "output sha256 instead of plain response")
	flag.IntVar(&sleep, "sleep", 0, "sleep number of milliseconds per request")

	flag.IntVar(&loops, "loops", 0, "number of accept loops (defaults to GOMAXPROCS)")
	flag.IntVar(&wpShards, "wpshards", 0, "number of workerpool shards (defaults to number of accept loops)")

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

	for i := 0; i < 0; i++ {
		go func() {
			server, _ := tcpserver.NewServer(listenAddr)
			server.SetListenConfig(&tcpserver.ListenConfig{
				SocketReusePort:   true,
				SocketFastOpen:    false,
				SocketDeferAccept: false,
			})
			server.SetRequestHandler(requestHandlerSimple)
			server.SetLoops(loops)
			server.SetWorkerpoolShards(wpShards)
			err := server.Listen()
			if err != nil {
				panic("Error listening on interface: " + err.Error())
			}
			err = server.Serve()
			if err != nil {
				panic("Error serving: " + err.Error())
			}

		}()
	}

	server, _ := tcpserver.NewServer(listenAddr)
	server.SetListenConfig(&tcpserver.ListenConfig{
		SocketReusePort:   true,
		SocketFastOpen:    false,
		SocketDeferAccept: false,
	})
	server.SetRequestHandler(requestHandlerSimple)
	server.SetLoops(loops)
	server.SetWorkerpoolShards(wpShards)
	err := server.Listen()
	if err != nil {
		panic("Error listening on interface: " + err.Error())
	}
	err = server.Serve()
	if err != nil {
		panic("Error serving: " + err.Error())
	}
}

func acquireReader(conn *tcpserver.Connection) *bufio.Reader {
	v:= brPool.Get()
	if v == nil {
		return bufio.NewReader(conn)
	}
	br := v.(*bufio.Reader)
	br.Reset(conn)
	return br
}

func releaseReader(br *bufio.Reader) {
	brPool.Put(br)
}

func acquireWriter(conn *tcpserver.Connection) *bufio.Writer {
	v:= bwPool.Get()
	if v == nil {
		return bufio.NewWriter(conn)
	}
	bw := v.(*bufio.Writer)
	bw.Reset(conn)
	return bw
}

func releaseWriter(bw *bufio.Writer) {
	bwPool.Put(bw)
}

func requestHandler(conn *tcpserver.Connection) {
	var leftover []byte
	var bw *bufio.Writer

	rv := reqVarsPool.Get().(*reqVars)

	bufSize := 0
	var buf [50]byte
	for {
		n, err := conn.Read(rv.data[bufSize:2048])
		if err != nil && n == 0 || n == 0 {
			break
		}
		bufSize += n

		leftover, err = parsereq(rv.data[0:bufSize], &rv.req)
		if err != nil {
			// bad thing happened
			if bw == nil {
				bw = acquireWriter(conn)
			}
			appendrespbw(bw, buf[:0], status500Error, []byte(err.Error()+"\n"))
			bw.Flush()
			break
		}

		if len(leftover) == len(rv.data) {
			// request not ready, yet
			continue
		}
		// handle the request
		if bw == nil {
			bw = acquireWriter(conn)
		}
		if aes128 {
			cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
			appendrespbw(bw, buf[:0], status200Ok, cryptedResbytes)
		} else if sha {
			sha256sum := sha256.Sum256(resbytes)
			appendrespbw(bw, buf[:0], status200Ok, []byte(hex.EncodeToString(sha256sum[:])))
		} else {
			appendrespbw(bw, buf[:0], status200Ok, resbytes)
		}

		if sleep > 0 {
			time.Sleep(time.Millisecond * time.Duration(sleep))
		}

		bw.Flush()

		if !keepAlive {
			break
		}

		bufSize = 0
	}

	if bw != nil {
		releaseWriter(bw)
	}

	reqVarsPool.Put(rv)
	return
}

func requestHandlerSimple(conn *tcpserver.Connection) {
	var leftover []byte

	rv := reqVarsPool.Get().(*reqVars)

	bufSize := 0
	var buf [50]byte
	for {
		n, err := conn.Read(rv.data[bufSize:2048])
		if err != nil && n == 0 || n == 0 {
			break
		}
		bufSize += n

		leftover, err = parsereq(rv.data[0:bufSize], &rv.req)
		if err != nil {
			// bad thing happened
			writeResponse(conn, rv.out[:0], buf[:0], status500Error, []byte(err.Error()+"\n"))
			break
		}

		if len(leftover) == len(rv.data) {
			// request not ready, yet
			continue
		}
		// handle the request
		if aes128 {
			cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
			writeResponse(conn, rv.out[:0], buf[:0], status200Ok, cryptedResbytes)
		} else if sha {
			sha256sum := sha256.Sum256(resbytes)
			writeResponse(conn, rv.out[:0], buf[:0], status200Ok, []byte(hex.EncodeToString(sha256sum[:])))
		} else {
			writeResponse(conn, rv.out[:0], buf[:0], status200Ok, resbytes)
		}

		if sleep > 0 {
			time.Sleep(time.Millisecond * time.Duration(sleep))
		}

		if !keepAlive {
			break
		}

		bufSize = 0
	}

	reqVarsPool.Put(rv)
	return
}

var headerHTTP11 = []byte("HTTP/1.1 ")
var headerDate = []byte("Date: ")
var headerConnectionClose = []byte("Connection: close")
var headerConnectionKeepAlive = []byte("Connection: keep-alive")
var headerServerIdentity = []byte("Server: tsrv")
var headerContentLength = []byte("Content-Length: ")
var headerContentType = []byte("Content-Type: ")
var headerContentTypeTextPlain = []byte("text/plain")
var newLine = []byte("\r\n")

// writeResponse will append a valid http response to the provide bytes.
// The status param should be the code plus text such as "200 OK".
// The head parameter should be a series of lines ending with "\r\n" or empty.
func writeResponse(w io.Writer, b, buf, status, body []byte) {
	b = append(b, headerHTTP11...)
	b = append(b, status...)
	b = append(b, newLine...)
	b = append(b, headerServerIdentity...)
	b = append(b, newLine...)
	if !keepAlive {
		b = append(b, headerConnectionClose...)
		b = append(b, newLine...)
	} else {
		b = append(b, headerConnectionKeepAlive...)
		b = append(b, newLine...)
	}
	b = append(b, headerDate...)
	b = append(b, serverDate.Load().([]byte)...)
	b = append(b, newLine...)
	if len(body) > 0 {
		b = append(b, headerContentType...)
		b = append(b, headerContentTypeTextPlain...)
		b = append(b, newLine...)
		b = append(b, headerContentLength...)
		b = append(b, AppendUint(buf[:0], len(body))...)
		//b = strconv.AppendInt(b, int64(len(body)), 10)
		b = append(b, newLine...)
	}
	b = append(b, newLine...)
	if len(body) > cap(b)-len(b) {
		w.Write(b)
		w.Write(body)
		return
	}
	b = append(b, body...)
	w.Write(b)
}

func appendrespbw(bw *bufio.Writer, buf[]byte, status, body []byte) {
	bw.Write(headerHTTP11)
	bw.Write(status)
	bw.Write(newLine)
	bw.Write(headerServerIdentity)
	bw.Write(newLine)
	if !keepAlive {
		bw.Write(headerConnectionClose)
		bw.Write(newLine)
	} else {
		bw.Write(headerConnectionKeepAlive)
		bw.Write(newLine)
	}
	bw.Write(headerDate)
	bw.Write(serverDate.Load().([]byte))
	bw.Write(newLine)

	if len(body) > 0 {
		bw.Write(headerContentType)
		bw.Write(headerContentTypeTextPlain)
		bw.Write(newLine)
		bw.Write(headerContentLength)
		bw.Write(AppendUint(buf[:0], len(body)))
		bw.Write(newLine)
	}
	bw.Write(newLine)
	if len(body) > 0 {
		bw.Write(body)
	}
}

// AppendUint appends n to dst and returns the extended dst.
func AppendUint(dst []byte, n int) []byte {
	if n < 0 {
		panic("BUG: int must be positive")
	}

	var b [20]byte
	buf := b[:]
	i := len(buf)
	var q int
	for n >= 10 {
		i--
		q = n / 10
		buf[i] = '0' + byte(n-q*10)
		n = q
	}
	i--
	buf[i] = '0' + byte(n)

	dst = append(dst, buf[i:]...)
	return dst
}

func appendTime(b []byte, t time.Time) []byte {
	const days = "SunMonTueWedThuFriSat"
	const months = "JanFebMarAprMayJunJulAugSepOctNovDec"

	t = t.UTC()
	yy, mm, dd := t.Date()
	hh, mn, ss := t.Clock()
	day := days[3*t.Weekday():]
	mon := months[3*(mm-1):]

	return append(b,
		day[0], day[1], day[2], ',', ' ',
		byte('0'+dd/10), byte('0'+dd%10), ' ',
		mon[0], mon[1], mon[2], ' ',
		byte('0'+yy/1000), byte('0'+(yy/100)%10), byte('0'+(yy/10)%10), byte('0'+yy%10), ' ',
		byte('0'+hh/10), byte('0'+hh%10), ':',
		byte('0'+mn/10), byte('0'+mn%10), ':',
		byte('0'+ss/10), byte('0'+ss%10), ' ',
		'G', 'M', 'T')
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
