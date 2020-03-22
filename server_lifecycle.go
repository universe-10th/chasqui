package chasqui

import (
	"net"
	"sync"
)


// Error that tells when a server is already listening.
type AlreadyListeningError bool


// The error message.
func (AlreadyListeningError) Error() string {
	return "server already listening"
}


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
type ServerLifeCycle struct {
	mutex           sync.Mutex
	listener        *net.TCPListener
	onStart         func(*ServerLifeCycle, *net.TCPAddr)
	onAcceptSuccess func(*ServerLifeCycle, *net.TCPConn)
	onAcceptError   func(*ServerLifeCycle, error)
	onStop          func(*ServerLifeCycle)
}


// Runs the server lifecycle in a separate goroutine. The
// only job of this server is to run the accept loop and
// report any error being triggered.
func (server *ServerLifeCycle) Run(host string) (func(), error) {
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
		server.listener.Close()
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
		server.onStart(server, finalHost)
		for {
			select {
			case <-quit:
				break
			default:
				if conn, err := server.listener.Accept(); err != nil {
					server.onAcceptError(server, err)
				} else {
					server.onAcceptSuccess(server, conn.(*net.TCPConn))
				}
			}
		}
		server.onStop(server)
	}()
	return func() { quit<-1 }, nil
}