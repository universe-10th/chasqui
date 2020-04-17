package chasqui

import (
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


// The set of all the attendants in a server.
type Attendants map[*Attendant]bool


// Event reporting the server has started.
type ServerStartedEvent struct {
	Addr   *net.TCPAddr
}


// Event reporting the server encountered
// an error when accepting a connection.
type ServerAcceptFailedEvent error


// Event reporting the server has stopped.
type ServerStoppedEvent uint8


// A default teamwork of a dispatcher and all the
// spawned connections (workers). In most cases,
// this implementation will suffice, so this one
// is the basic server implementation. It will
// keep a list of current attendants, choose a
// marshaler factory to wrap the connections
// when spawning an attendant, and also connect
// the flows of the attendants to the flow of the
// dispatcher.
type Server struct {
	dispatcher            *Dispatcher
	attendants            Attendants
	startedEvent          chan ServerStartedEvent
	acceptFailedEvent     chan ServerAcceptFailedEvent
	attendantStartedEvent chan AttendantStartedEvent
	messageEvent          chan MessageEvent
	throttledEvent        chan ThrottledEvent
	attendantStoppedEvent chan AttendantStoppedEvent
	stoppedEvent          chan ServerStoppedEvent
	closer                func()
}


// Runs the server. This implies running the underlying
// dispatcher and relying on the callbacks to do their
// job.
func (server *Server) Run(host string) error {
	if closer, err := server.dispatcher.Run(host); err != nil {
		return err
	} else {
		server.closer = closer
		return nil
	}
}


// Stops the server, if running.
func (server *Server) Stop() error {
	if server.closer == nil {
		return DispatcherNotListeningError(true)
	} else {
		server.closer()
		server.Enumerate(func(attendant *Attendant) {
			// noinspection GoUnhandledErrorResult
			attendant.Stop()
		})
		server.attendants = Attendants{}
		server.closer = nil
		return nil
	}
}


// Returns a read-only channel with all the "started" events.
func (server *Server) StartedEvent() <-chan ServerStartedEvent {
	return server.startedEvent
}


// Returns a read-only channel with all the "accept failed" events.
func (server *Server) AcceptFailedEvent() <-chan ServerAcceptFailedEvent {
	return server.acceptFailedEvent
}


// Returns a read-only channel with all the "attendant started" events.
func (server *Server) AttendantStartedEvent() <-chan AttendantStartedEvent {
	return server.attendantStartedEvent
}


// Returns a read-only channel with all the received messages.
func (server *Server) MessageEvent() <-chan MessageEvent {
	return server.messageEvent
}


// Returns a read-only channel with all the "throttled" events.
func (server *Server) ThrottledEvent() <-chan ThrottledEvent {
	return server.throttledEvent
}


// Returns a read-only channel with all the "attendant stopped" events.
func (server *Server) AttendantStoppedEvent() <-chan AttendantStoppedEvent {
	return server.attendantStoppedEvent
}


// Returns a read-only channel with all the "stopped" events.
func (server *Server) StoppedEvent() <-chan ServerStoppedEvent {
	return server.stoppedEvent
}


// Returns the current listen address of the server,
// if running. Returns an error if it is not running.
func (server *Server) Addr() (net.Addr, error) {
	return server.dispatcher.Addr()
}


// Enumerates all the attendants using a callback. It will seldom
// be used - perhaps for lobby features or debugging purposes.
func (server *Server) Enumerate(callback func(*Attendant)) {
	for attendant, _ := range server.attendants {
		callback(attendant)
	}
}


// Creates a new server by configuring a marshaler factory, the channel buffer size for the
// message and throttled events, the default throttle time, and the buffer sizes.
func NewServer(factory MessageMarshaler, activityBufferSize, lifecycleBufferSize uint, defaultThrottle time.Duration) *Server {
	if factory == nil {
		panic(ArgumentError{"NewServer:factory"})
	}

	var onDispatcherAcceptError OnDispatcherAcceptError
	var onDispatcherStart OnDispatcherStart
	var onDispatcherStop OnDispatcherStop
	var onDispatcherAcceptSuccess OnDispatcherAcceptSuccess
	if activityBufferSize < 16 {
		activityBufferSize = 16
	}
	if lifecycleBufferSize < 1 {
		lifecycleBufferSize = 1
	}
	server := &Server{
		attendants:            Attendants{},
		startedEvent:          make(chan ServerStartedEvent, lifecycleBufferSize),
		acceptFailedEvent:     make(chan ServerAcceptFailedEvent, lifecycleBufferSize),
		attendantStartedEvent: make(chan AttendantStartedEvent, lifecycleBufferSize),
		messageEvent:          make(chan MessageEvent, activityBufferSize),
		throttledEvent:        make(chan ThrottledEvent, activityBufferSize),
		attendantStoppedEvent: make(chan AttendantStoppedEvent, lifecycleBufferSize),
		stoppedEvent:          make(chan ServerStoppedEvent, lifecycleBufferSize),
	}
	// Intermediate events from the attendants and the mapping
	// lifecycle the basic server implements.
	attendantStartedEvent := make(chan AttendantStartedEvent)
	attendantStoppedEvent := make(chan AttendantStoppedEvent)
	quit := make(chan uint8)

	onDispatcherStart = func(_dispatcher *Dispatcher, addr *net.TCPAddr) {
		go func(){
			Loop: for {
				select {
				case event := <- attendantStartedEvent:
					server.attendants[event.Attendant] = true
					server.attendantStartedEvent <- event
				case event := <- attendantStoppedEvent:
					delete(server.attendants, event.Attendant)
					server.attendantStoppedEvent <- event
				case <-quit:
					break Loop
				}
			}
		}()
		server.startedEvent <- ServerStartedEvent{
			Addr: addr,
		}
	}
	onDispatcherStop = func(_dispatcher *Dispatcher) {
		close(quit)
		server.stoppedEvent <- ServerStoppedEvent(1)
	}
	onDispatcherAcceptError = func(_dispatcher *Dispatcher, err error) {
		server.acceptFailedEvent <- ServerAcceptFailedEvent(err)
	}
    onDispatcherAcceptSuccess = func(dispatcher *Dispatcher, conn *net.TCPConn) {
		attendant := NewAttendant(
			conn, factory, defaultThrottle, attendantStartedEvent, attendantStoppedEvent,
			server.messageEvent, server.throttledEvent,
		)
		// noinspection GoUnhandledErrorResult
		attendant.Start()
	}
	server.dispatcher = NewDispatcher(onDispatcherStart, onDispatcherAcceptSuccess,
		                                   onDispatcherAcceptError, onDispatcherStop)
	return server
}


// Processes all the events of a Server as callbacks. It is guaranteed that
// all the callbacks will be run inside a single goroutine, preventing any
//kind of race conditions, when using this kind of objects.
type ServerFunnel interface {
	Started(*Server, *net.TCPAddr)
	AcceptFailed(*Server, error)
	Stopped(*Server)
	AttendantStarted(*Server, *Attendant)
	MessageArrived(*Server, *Attendant, Message)
	MessageThrottled(*Server, *Attendant, Message, time.Time, time.Duration)
	AttendantStopped(*Server, *Attendant, AttendantStopType, error)
}


// Creates a funnel: runs a goroutine dispatching all the events from a server
// to a given funnel object processing all the events. A funnel may be used by
// several servers, but care should be taken, for race conditions will not be
// prevented among different servers (yes among events inside the same server.
// A server, on the other hand, will not work appropriately if used by several
// funnels.
func FunnelServerWith(server *Server, funnel ServerFunnel) {
	if server == nil {
		panic(ArgumentError{"Funnel:server"})
	}
	if funnel == nil {
		panic(ArgumentError{"Funnel:funnel"})
	}

	go func(server *Server) {
		Loop: for {
			select {
			case event := <-server.StartedEvent():
				funnel.Started(server, event.Addr)
			case event := <-server.AcceptFailedEvent():
				funnel.AcceptFailed(server, event)
			case <-server.StoppedEvent():
				funnel.Stopped(server)
				break Loop
			case event := <-server.AttendantStartedEvent():
				funnel.AttendantStarted(server, event.Attendant)
			case event := <-server.MessageEvent():
				funnel.MessageArrived(server, event.Attendant, event.Message)
			case event := <-server.ThrottledEvent():
				funnel.MessageThrottled(server, event.Attendant, event.Message, event.Instant, event.Lapse)
			case event := <-server.AttendantStoppedEvent():
				funnel.AttendantStopped(server, event.Attendant, event.StopType, event.Error)
			}
		}
	}(server)
}