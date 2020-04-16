package chasqui

import (
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


// The set of all the attendants in a server.
type Attendants map[*Attendant]bool


// Event reporting the server has started.
type BasicServerStartedEvent struct {
	Addr   *net.TCPAddr
}


// Event reporting the server encountered
// an error when accepting a connection.
type BasicServerAcceptFailedEvent error


// Event reporting the server has stopped.
type BasicServerStoppedEvent uint8


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
	dispatcher            *Dispatcher
	attendants            Attendants
	startedEvent          chan BasicServerStartedEvent
	acceptFailedEvent     chan BasicServerAcceptFailedEvent
	attendantStartedEvent chan AttendantStartedEvent
	messageEvent          chan MessageEvent
	throttledEvent        chan ThrottledEvent
	attendantStoppedEvent chan AttendantStoppedEvent
	stoppedEvent          chan BasicServerStoppedEvent
	closer                func()
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
			// noinspection GoUnhandledErrorResult
			attendant.Stop()
		})
		basicServer.attendants = Attendants{}
		basicServer.closer = nil
		return nil
	}
}


// Returns a read-only channel with all the "started" events.
func (basicServer *BasicServer) StartedEvent() <-chan BasicServerStartedEvent {
	return basicServer.startedEvent
}


// Returns a read-only channel with all the "accept failed" events.
func (basicServer *BasicServer) AcceptFailedEvent() <-chan BasicServerAcceptFailedEvent {
	return basicServer.acceptFailedEvent
}


// Returns a read-only channel with all the "attendant started" events.
func (basicServer *BasicServer) AttendantStartedEvent() <-chan AttendantStartedEvent {
	return basicServer.attendantStartedEvent
}


// Returns a read-only channel with all the received messages.
func (basicServer *BasicServer) MessageEvent() <-chan MessageEvent {
	return basicServer.messageEvent
}


// Returns a read-only channel with all the "throttled" events.
func (basicServer *BasicServer) ThrottledEvent() <-chan ThrottledEvent {
	return basicServer.throttledEvent
}


// Returns a read-only channel with all the "attendant stopped" events.
func (basicServer *BasicServer) AttendantStoppedEvent() <-chan AttendantStoppedEvent {
	return basicServer.attendantStoppedEvent
}


// Returns a read-only channel with all the "stopped" events.
func (basicServer *BasicServer) StoppedEvent() <-chan BasicServerStoppedEvent {
	return basicServer.stoppedEvent
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


// Creates a new server by configuring a marshaler factory, the channel buffer size for the
// message and throttled events, the default throttle time, and the buffer sizes.
func NewServer(factory MessageMarshaler, activityBufferSize, lifecycleBufferSize uint, defaultThrottle time.Duration) *BasicServer {
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
	basicServer := &BasicServer{
		attendants:            Attendants{},
		startedEvent:          make(chan BasicServerStartedEvent, lifecycleBufferSize),
		acceptFailedEvent:     make(chan BasicServerAcceptFailedEvent, lifecycleBufferSize),
		attendantStartedEvent: make(chan AttendantStartedEvent, lifecycleBufferSize),
		messageEvent:          make(chan MessageEvent, activityBufferSize),
		throttledEvent:        make(chan ThrottledEvent, activityBufferSize),
		attendantStoppedEvent: make(chan AttendantStoppedEvent, lifecycleBufferSize),
		stoppedEvent:          make(chan BasicServerStoppedEvent, lifecycleBufferSize),
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
					basicServer.attendants[event.Attendant] = true
					basicServer.attendantStartedEvent <- event
				case event := <- attendantStoppedEvent:
					delete(basicServer.attendants, event.Attendant)
					basicServer.attendantStoppedEvent <- event
				case <-quit:
					break Loop
				}
			}
		}()
		basicServer.startedEvent <- BasicServerStartedEvent{
			Addr: addr,
		}
	}
	onDispatcherStop = func(_dispatcher *Dispatcher) {
		close(quit)
		basicServer.stoppedEvent <- BasicServerStoppedEvent(1)
	}
	onDispatcherAcceptError = func(_dispatcher *Dispatcher, err error) {
		basicServer.acceptFailedEvent <- BasicServerAcceptFailedEvent(err)
	}
    onDispatcherAcceptSuccess = func(dispatcher *Dispatcher, conn *net.TCPConn) {
		attendant := NewAttendant(
			conn, factory, defaultThrottle, attendantStartedEvent, attendantStoppedEvent,
			basicServer.messageEvent, basicServer.throttledEvent,
		)
		// noinspection GoUnhandledErrorResult
		attendant.Start()
	}
	basicServer.dispatcher = NewDispatcher(onDispatcherStart, onDispatcherAcceptSuccess,
		                                   onDispatcherAcceptError, onDispatcherStop)
	return basicServer
}


// Processes all the events of a Basic Server as callbacks. It is guaranteed
// that all the callbacks will be run inside a single goroutine, preventing
// any kind of race conditions, when using this kind of objects.
type BasicServerFunnel interface {
	Started(*BasicServer, *net.TCPAddr)
	AcceptFailed(*BasicServer, error)
	Stopped(*BasicServer)
	AttendantStarted(*BasicServer, *Attendant)
	MessageArrived(*BasicServer, *Attendant, Message)
	MessageThrottled(*BasicServer, *Attendant, Message, time.Time, time.Duration)
	AttendantStopped(*BasicServer, *Attendant, AttendantStopType, error)
}


// Creates a funnel: runs a goroutine dispatching all the events from a server
// to a given funnel object processing all the events. A funnel may be used by
// several servers, but care should be taken, for race conditions will not be
// prevented among different servers (yes among events inside the same server.
// A server, on the other hand, will not work appropriately if used by several
// funnels.
func ServerFunnel(server *BasicServer, funnel BasicServerFunnel) {
	if server == nil {
		panic(ArgumentError{"Funnel:server"})
	}
	if funnel == nil {
		panic(ArgumentError{"Funnel:funnel"})
	}

	go func(basicServer *BasicServer) {
		Loop: for {
			select {
			case event := <-basicServer.StartedEvent():
				funnel.Started(server, event.Addr)
			case event := <-basicServer.AcceptFailedEvent():
				funnel.AcceptFailed(basicServer, event)
			case <-basicServer.StoppedEvent():
				funnel.Stopped(basicServer)
				break Loop
			case event := <-basicServer.AttendantStartedEvent():
				funnel.AttendantStarted(basicServer, event.Attendant)
			case event := <-basicServer.MessageEvent():
				funnel.MessageArrived(basicServer, event.Attendant, event.Message)
			case event := <-basicServer.ThrottledEvent():
				funnel.MessageThrottled(basicServer, event.Attendant, event.Message, event.Instant, event.Lapse)
			case event := <-basicServer.AttendantStoppedEvent():
				funnel.AttendantStopped(basicServer, event.Attendant, event.StopType, event.Error)
			}
		}
	}(server)
}