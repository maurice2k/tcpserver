package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func main() {
	var res string
	var listenAddr string
	var aaaa int
	var keepAlive bool
	var aes128 bool
	var sha bool
	var sleep int

	var aesKey = []byte("0123456789ABCDEF")

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

	resbytes := []byte(res)

	log.Printf("http server using valyala/fasthttp starting on %s with GOMAXPROCS=%d", listenAddr, runtime.GOMAXPROCS(0))
	s := &fasthttp.Server{
		Handler: func(c *fasthttp.RequestCtx) {

			if aes128 {
				cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
				c.Write(cryptedResbytes)
			} else if sha {
				sha256sum := sha256.Sum256(resbytes)
				c.WriteString(hex.EncodeToString(sha256sum[:]))
			} else {
				c.Write(resbytes)
			}

			if sleep > 0 {
				time.Sleep(time.Millisecond * time.Duration(sleep))
			}
		},
		DisableKeepalive: !keepAlive,
	}
	s.ListenAndServe(listenAddr)
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
