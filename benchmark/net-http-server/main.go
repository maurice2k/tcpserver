package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

func main() {
	var res string
	var listenAddr string
	var aaaa int
	var keepAlive bool
	var aes128 bool
	var sha bool
	var sleep int
	var useTls bool

	var aesKey = []byte("0123456789ABCDEF")

	flag.StringVar(&listenAddr, "listen", "127.0.0.1:8000", "server listen addr")
	flag.IntVar(&aaaa, "aaaa", 0, "aaaaa.... (default output is 'Hello World')")
	flag.BoolVar(&keepAlive, "keepalive", true, "use HTTP Keep-Alive")
	flag.BoolVar(&aes128, "aes128", false, "encrypt response with aes-128-cbc")
	flag.BoolVar(&sha, "sha", false, "output sha256 instead of plain response")
	flag.IntVar(&sleep, "sleep", 0, "sleep number of milliseconds per request")
	flag.BoolVar(&useTls, "useTls", false, "use HTTPS")
	flag.Parse()

	if aaaa > 0 {
		res = strings.Repeat("a", aaaa)
	} else {
		res = "Hello World!\r\n"
	}

	resbytes := []byte(res)

	log.Printf("http server using plain golang net/* starting on %s with GOMAXPROCS=%d", listenAddr, runtime.GOMAXPROCS(0))
	s := &http.Server{
		Addr: listenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// handle the request
			if aes128 {
				cryptedResbytes, _ := encryptCBC(resbytes, aesKey)
				w.Write(cryptedResbytes)
			} else if sha {
				sha256sum := sha256.Sum256(resbytes)
				w.Write([]byte(hex.EncodeToString(sha256sum[:])))
			} else {
				w.Write(resbytes)
			}

			if sleep > 0 {
				time.Sleep(time.Millisecond * time.Duration(sleep))
			}
		}),
	}

	s.SetKeepAlivesEnabled(keepAlive)

	var err error
	if useTls {
		s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{getCert()}}
		err = s.ListenAndServeTLS("", "")
	} else {
		err = s.ListenAndServe()
	}
	if err != nil {
		log.Fatal(err)
	}
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


// Returns testing cert
func getCert() (cert tls.Certificate) {
	certPem := `
-----BEGIN CERTIFICATE-----
MIIFgzCCA2ugAwIBAgIUOAG3o6IsqyYwaSecWpft29luvD0wDQYJKoZIhvcNAQEL
BQAwUTELMAkGA1UEBhMCREUxEzARBgNVBAgMClNvbWUtU3RhdGUxDzANBgNVBAcM
Bk11bmljaDEcMBoGA1UECgwTbWF1cmljZTJrL3RjcHNlcnZlcjAeFw0yMTAyMDgw
OTUyMTBaFw0zMTAyMDYwOTUyMTBaMFExCzAJBgNVBAYTAkRFMRMwEQYDVQQIDApT
b21lLVN0YXRlMQ8wDQYDVQQHDAZNdW5pY2gxHDAaBgNVBAoME21hdXJpY2Uyay90
Y3BzZXJ2ZXIwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDO043SaQoE
QNSEALBnG/1qPLvwSwe+JDC11ebRBhaWvYLzycwzDK3IewM8Oa2ygqCLi1MhV8TX
qfuu5R0+OFwyp4tBGyTmtcyg4k7HK7lrtq8jVlLzyVmg5k3g9RR4ab+aiAc7R54T
DcR2kLm7Xl8Jn2XJhKlyneK2HMufxmUh5EF2S5jMsHh0b8yrbmfio1Dxi3QZGDrs
QHULPZ47TbcC1B790Z8bVnfzOmFYJUF92H8l2utAb0q0ARHImPRJOjwW7TOYIWbi
QYI4aE4Te2zq4V26qjEcP/IWFVxNFg7+1uSrb4RlyjTKoKvSGlYj/hDitQOheOIg
XDqKyEs3yxfQOATsUE8/J26SGTnwauBRblrZBYi8jrHDm+FJcmc65/dsAZe42wCd
oTs7k9gV0CvjXvvXRITr/YkRA7epYfEErVHl112wZ30p6T+YznPiBh8xNbijWlcH
T/mER0TaGX8vzyTj/Dy1fY0oQhaP79LwAVbUgTtMBv7bwtrH4xX+kvBm5j5NLrUS
diXmeFYB6H1ZUFzEnlIsICs5rb1fCvJlSbQxwq6fqNkZxyZU3e9JxMzQ8pgDmrKg
KPmxDsm/7sX3tCKX7o9Fd6PH4rlEsWQxMM7/1mINgR0SkdRLZCogybvFELrWFLdb
bmlZc52FqSIvMnj8fTfG6rxNVJ8A6pLd3QIDAQABo1MwUTAdBgNVHQ4EFgQUo1A/
GiyZkQEnTbtyvVJl9D1qFHowHwYDVR0jBBgwFoAUo1A/GiyZkQEnTbtyvVJl9D1q
FHowDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAgEAjQGOL7fxT4bT
eAjbIZLzbSX7euvJsQAQJHikI5ZY9ROnFlx1N26bGK28OaqbaW6bqkPcoRm8qWSV
+xqYiEx0ebKnGRf5OppRjgg9DmOL3n9PiYpC/dJBkPg2V4F7iFGL4YQJnHsNRl4v
Ke6YO2qCI430WwOLY/69imkOc+ob+G3GYt0Oim58z+SRFU4eUwiYxQqCZaNVAEV5
IQg5QUWOgT5kSI0e3HK7QgutlMP3AhawMACXfPWM+iN3v7DJk8mDAbh0cCWRi8PG
Q7Ms+hR8Vx+CiekPO/S2TgvWiBYvsF8QJ2Cyg6x7rXTwFBSMaDvBYQ5CIPYxQMng
B6L4z1SC9o9pFomwU8W/BrloUEUeTe66YeL4v+Yy9brXVM7nK0U0qxhPpIH39oFS
5v/k9JZ071nZbpdr6P1E55xCEFbB18f6ljYQ55xNpjFMzuvYWy89bTjA+M7q47t5
2PXFal8i++Z8jqfhUvOxidf/EqQ93GFCzchS2Zf3ut9nqmQYz1zWqFY7GHO0UIuH
DDXnt9CZduCL5Jpc8J6kITuO+2MWrgLd2OoCNZLOhD/yWPQcSjA6C+bDc0MN/TtT
Y6UOlBoByevGWAejLP2XjNJELr1VkTgv/sYXoFazDggNIovSWNTmcoSJJ6Zmy4Zo
V3Dl12p/TQ3/eu5v7x3D7zBcwluxrvI=
-----END CERTIFICATE----- 
`
	keyPem := `
-----BEGIN PRIVATE KEY-----
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
3uj4+nXPk8tk7guNm0WV0n8KBKwR
-----END PRIVATE KEY----- 
`
	cert, _ = tls.X509KeyPair([]byte(certPem), []byte(keyPem))
	return
}
