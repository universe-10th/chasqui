package chasqui

import (
	"net"
	"time"
)


// The set of all the attendants in a server.
type Attendants map[*Attendant]bool


// Callback to report when a server successfully ran
// its lifecycle.
type OnBasicServerStart func(*BasicServer, *net.TCPAddr)


// Callback to report when a server failed to accept
// an incoming connection.
type OnBasicServerAcceptError func(*BasicServer, error)


// Callback to report when a server successfully ended
// its lifecycle.
type OnBasicServerStop func(*BasicServer)


// A default teamwork of a dispatcher and all the
// spawned connections (workers). In most cases,
// this implementation will suffice, so this one
// is the basic server implementation. It will
// keep a list of current attendants, choose a
// marshaler factory to wrap the connections
// when spawning an attendant, and also connect
// the flows of the attendants to the flow of the
// dispatcher.
type BasicServer struct {
	dispatcher *Dispatcher
	attendants Attendants
	conveyor   chan Conveyed
	closer     func()
}


// Runs the server. This implies running the underlying
// dispatcher and relying on the callbacks to do their
// job.
func (basicServer *BasicServer) Run(host string) error {
	if closer, err := basicServer.dispatcher.Run(host); err != nil {
		return err
	} else {
		basicServer.closer = closer
		return nil
	}
}


// Stops the server, if running.
func (basicServer *BasicServer) Stop() error {
	if basicServer.closer == nil {
		return DispatcherNotListeningError(true)
	} else {
		basicServer.closer()
		basicServer.Enumerate(func(attendant *Attendant) {
			_ = attendant.Stop()
		})
		basicServer.attendants = Attendants{}
		basicServer.closer = nil
		return nil
	}
}


// Returns a read-only channel with all the conveyed messages.
func (basicServer *BasicServer) Conveyor() <-chan Conveyed {
	return basicServer.conveyor
}


// Returns the current listen address of the server,
// if running. Returns an error if it is not running.
func (basicServer *BasicServer) Addr() (net.Addr, error) {
	return basicServer.dispatcher.Addr()
}


// Enumerates all the attendants using a callback. It will seldom
// be used - perhaps for lobby features or debugging purposes.
func (basicServer *BasicServer) Enumerate(callback func(*Attendant)) {
	for attendant, _ := range basicServer.attendants {
		callback(attendant)
	}
}


// Creates a new server by configuring a marshaler factory, the conveyor buffer size,
// the default throttle time, and all the callbacks.
func NewServer(factory MessageMarshaler, conveyorBufferSize int, defaultThrottle time.Duration,
	           onServerStart OnBasicServerStart, onServerAcceptError OnBasicServerAcceptError,
	           onServerStop OnBasicServerStop, onAttendantStart OnAttendantStart,
	           onAttendantStop OnAttendantStop, onAttendantThrottle OnAttendantThrottle) *BasicServer {
	var onDispatcherAcceptError OnDispatcherAcceptError
	var onDispatcherStart OnDispatcherStart
	var onDispatcherStop OnDispatcherStop
	var onDispatcherAcceptSuccess OnDispatcherAcceptSuccess
	basicServer := &BasicServer{
		attendants: Attendants{},
		conveyor:   make(chan Conveyed, conveyorBufferSize),
	}
	if onServerStart != nil {
		onDispatcherStart = func(_dispatcher *Dispatcher, addr *net.TCPAddr) {
			onServerStart(basicServer, addr )
		}
	}
	if onServerStop != nil {
		onDispatcherStop = func(_dispatcher *Dispatcher) {
			onServerStop(basicServer)
		}
	}
	if onServerAcceptError != nil {
		onDispatcherAcceptError = func(_dispatcher *Dispatcher, err error) {
			onServerAcceptError(basicServer, err)
		}
	}
	onAttendantStopWrapper := func(attendant *Attendant, stopType AttendantStopType, err error) {
		delete(basicServer.attendants, attendant)
		if onAttendantStop != nil {
			onAttendantStop(attendant, stopType, err)
		}
	}
	onDispatcherAcceptSuccess = func(dispatcher *Dispatcher, conn *net.TCPConn) {
		attendant := NewAttendant(
			conn, factory, basicServer.conveyor, defaultThrottle,
			onAttendantStart, onAttendantStopWrapper, onAttendantThrottle,
		)
		basicServer.attendants[attendant] = true
		_ = attendant.Start()
	}
	basicServer.dispatcher = NewDispatcher(onDispatcherStart, onDispatcherAcceptSuccess,
		                                   onDispatcherAcceptError, onDispatcherStop)
	return basicServer
}