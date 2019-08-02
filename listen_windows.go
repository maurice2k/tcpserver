// Copyright 2019 Moritz Fain
// Moritz Fain <moritz@fain.io>

// +build windows

package tcpserver

import (
	"fmt"
	"syscall"
)

const (
	tcpFastOpen = 0x17
)

type controlFunc func(network, address string, c syscall.RawConn) error

func applyListenSocketOptions(lc *ListenConfig) controlFunc {
	return func(network, address string, c syscall.RawConn) error {
		var err error
		c.Control(func(fd uintptr) {
			if lc.SocketFastOpen {
				qlen := lc.SocketFastOpenQueueLen
				if qlen <= 0 {
					qlen = 256
				}
				err = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, tcpFastOpen, qlen)
				if err != nil {
					err = fmt.Errorf("unable to set TCP_FASTOPEN option: %s", err)
				}
			}
		})
		return err
	}
}
