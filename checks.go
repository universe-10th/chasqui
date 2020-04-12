package chasqui

import "net"

var dummyMessage = []byte{1}
var errNetClosing error


// Grabs the underlying ErrNetClosing error, using a
// workaround since "internal/poll" cannot be imported
// to get/compare the poll.ErrNetClosing error.
func ErrNetClosing() error {
	if errNetClosing == nil {
		var dummyServer *net.TCPListener
		if addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0"); err != nil {
			return nil
		} else if dummyServer, err = net.ListenTCP("tcp", addr); err != nil {
			return nil
		} else {
			// noinspection GoUnhandledErrorResult
			defer dummyServer.Close()
		}
		if dummyClient, err := net.DialTCP("tcp", nil, dummyServer.Addr().(*net.TCPAddr)); err != nil {
			return nil
		} else {
			// noinspection GoUnhandledErrorResult
			dummyClient.Close()
			_, err = dummyClient.Write(dummyMessage)
			if opError, ok := err.(*net.OpError); ok {
				errNetClosing = opError.Err
			}
		}
	}
	return errNetClosing
}
