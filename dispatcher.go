package chasqui

import (
	"net"
	"sync"
)


// Error that tells when a server is already listening.
type DispatcherAlreadyListeningError bool


// The error message.
func (DispatcherAlreadyListeningError) Error() string {
	return "server already listening"
}


// Callback to report when a dispatcher successfully ran
// its lifecycle.
type OnDispatcherStart func(*Dispatcher, *net.TCPAddr)


// Callback to report when an dispatcher could successfully
// accept an incoming connection.
type OnDispatcherAcceptSuccess func(*Dispatcher, *net.TCPConn)


// Callback to report when an dispatcher failed to accept
// an incoming connection.
type OnDispatcherAcceptError func(*Dispatcher, error)


// Callback to report when an dispatcher successfully ended
// its lifecycle.
type OnDispatcherStop func(*Dispatcher)


// A server lifecycle for TCP sockets. It does not provide
// any mean or workflow for the individual connections. It
// provides 4 callbacks to handle when it started, when it
// closed, when it accepted a connection or when it failed
// to accept a connection.
//
// When invoking its Run method, it will return either an
// error or a "closer" function: a function with no args /
// return value that will close the server. This implies
// that the lifecycle will run on its own goroutine.
type Dispatcher struct {
	mutex           sync.Mutex
	listener        *net.TCPListener
	onStart         OnDispatcherStart
	onAcceptSuccess OnDispatcherAcceptSuccess
	onAcceptError   OnDispatcherAcceptError
	onStop          OnDispatcherStop
}


// Runs the server lifecycle in a separate goroutine. The
// only job of this server is to run the accept loop and
// report any error being triggered.
func (server *Dispatcher) Run(host string) (func(), error) {
	// Start to listen, and keep the listener.
	var finalHost *net.TCPAddr
	server.mutex.Lock()
	if host, errHost := net.ResolveTCPAddr("tcp", host); errHost != nil {
		return nil, errHost
	} else if listener, errListen := net.ListenTCP("tcp", host); errListen != nil {
		return nil, errListen
	} else {
		finalHost = host
		server.listener = listener
	}
	server.mutex.Unlock()

	// Create the channel to send the quit signal.
	quit := make(chan uint8)

	// Launch the goroutine. Such goroutine will
	// be stopped by the quit signal. Listeners will
	// never report when they are closed, since they
	// got accepted the first time. The only way to
	// stop them, is gracefully.
	go func(){
		if server.onStart != nil {
			server.onStart(server, finalHost)
		}
		for {
			select {
			case <-quit:
				break
			default:
				if conn, err := server.listener.Accept(); err != nil {
					if server.onAcceptError != nil {
						server.onAcceptError(server, err)
					}
				} else {
					if server.onAcceptSuccess != nil {
						server.onAcceptSuccess(server, conn.(*net.TCPConn))
					}
				}
			}
		}
		if server.onStop != nil {
			server.onStop(server)
		}
		server.listener.Close()
		server.listener = nil
	}()
	return func() { quit<- 1 }, nil
}


// Creates a new dispatcher, ready to be used.
func NewDispatcher(onStart OnDispatcherStart, onAcceptSuccess OnDispatcherAcceptSuccess,
				   onAcceptError OnDispatcherAcceptError, onStop OnDispatcherStop) *Dispatcher {
	return &Dispatcher{
		onStart: onStart,
		onAcceptSuccess: onAcceptSuccess,
		onAcceptError: onAcceptError,
		onStop: onStop,
	}
}