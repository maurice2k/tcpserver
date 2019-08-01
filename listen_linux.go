// Copyright 2019 Moritz Fain
// Moritz Fain <moritz@fain.io>

// +build linux

package tcpserver

import (
	"fmt"
	"syscall"
)

const (
	soReusePort = 0x0F
	tcpFastOpen = 0x17
)

type controlFunc func(network, address string, c syscall.RawConn) error

func applyListenSocketOptions(lc *ListenConfig) controlFunc {
	return func(network, address string, c syscall.RawConn) error {
		var err error
		c.Control(func(fd uintptr) {

			if lc.SocketReusePort {
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, soReusePort, 1)
				if err != nil {
					err = fmt.Errorf("unable to set SO_REUSEPORT option: %s", err)
				}
			}
			if lc.SocketFastOpen {
				qlen := lc.SocketFastOpenQueueLen
				if qlen <= 0 {
					qlen = 1024
				}
				err = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpFastOpen, qlen)
				if err != nil {
					err = fmt.Errorf("unable to set TCP_FASTOPEN option: %s", err)
				}
			}

			if lc.SocketDeferAccept {
				err = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_DEFER_ACCEPT, 1)
				if err != nil {
					err = fmt.Errorf("unable to set TCP_DEFER_ACCEPT option: %s", err)
				}
			}

		})
		return err
	}
}
