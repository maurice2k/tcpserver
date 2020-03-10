// Copyright 2019 Moritz Fain
// Moritz Fain <moritz@fain.io>

// +build !linux,!windows

package tcpserver

import "syscall"

type controlFunc func(network, address string, c syscall.RawConn) error

func applyListenSocketOptions(lc *ListenConfig) controlFunc {
	return nil
}