// Copyright 2019-2022 Moritz Fain
// Moritz Fain <moritz@fain.io>

//go:build !linux && !windows
// +build !linux,!windows

package tcpserver

import "syscall"

type controlFunc func(network, address string, c syscall.RawConn) error

func applyListenSocketOptions(lc *ListenConfig) controlFunc {
	return nil
}
