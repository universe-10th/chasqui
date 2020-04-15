package chasqui

import (
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


// Error that tells when an attendant is not in a New state.
type AttendantIsNotNew bool


// The error message.
func (AttendantIsNotNew) Error() string {
	return "attendant cannot start - is either running or closed"
}

// Error that tells when an attendant is in a Stopped state.
type AttendantIsStopped bool


// The error message.
func (AttendantIsStopped) Error() string {
	return "attendant cannot send any message - it is stopped"
}


// Error that tells when an attendant is already in a Stopped state.
type AttendantIsAlreadyStopped bool


// The error message.
func (AttendantIsAlreadyStopped) Error() string {
	return "attendant cannot be closed - it is already stopped"
}


// The status of an Attendant. It will have 3 sequential
// internal states:
// - New: The attendant was just created, but not yet started.
// - Running: The attendant is running.
// - Closed: The attendant is closed, or was just told to close.
type AttendantStatus int
const (
	AttendantNew = iota
	AttendantRunning
	AttendantStopped
)


// An indicator telling the type of sub-event for the close event.
type AttendantStopType int
const (
	AttendantLocalStop = iota
	AttendantRemoteStop
	AttendantAbnormalStop
)


// Start events come in a dummy structure with the attendant as
// the only value.
type AttendantStartedEvent struct {
	Attendant *Attendant
}


// Messages being conveyed come in another kind of structure: The
// structure will hold both the conveyed message and the attendant
// that received and conveyed it.
type MessageEvent struct {
	Attendant *Attendant
	Message   Message
}


// AttendantStoppedEvent events come in another kind of structure: The structure
// will hold the attendant just stopped, the stop kind, and the
// error object for abnormal stops. AttendantStoppedEvent attendants are totally
// useless as they are already closed, and so the handling of this
// event should not attempt any further interaction with any of the
// socket features of the attendant.
type AttendantStoppedEvent struct {
	Attendant *Attendant
	StopType  AttendantStopType
	Error     error
}


// ThrottledEvent events come in another kind of structure: The structure
// will hold the attendant receiving the throttled message, the
// instant of the throttle, and the message itself.
type ThrottledEvent struct {
	Attendant *Attendant
	Message   Message
	Instant   time.Time
	Lapse     time.Duration
}


// Attendants are spawned objects and routines for a single
// incoming connection. They are created using certain protocol
// factory (an instance of MessageMarshaler), are connected to
// a central message channel (one for all the bunch of messages)
// to read the incoming messages via an individual goroutine that
// works as a "read loop", and have methods to Write and Close
// the attendant (and the underlying connection).
//
// When an attendant is started, right before the read loop is
// started, an "on start" event will be triggered. The new
// connection is available to write, but nothing will be read
// at this moment. Then, the read loop is started. It may have
// a throttle being configured to avoid abuse from the remote
// endpoint.
//
// Everything will keep working until one of these occurs:
// - The attendant is told to close.
// - An error occurs in the read loop.
//
// In the first case, the connection is told to close. Knowing
// this, one can completely ignore the read loop. After telling
// the connection to close, an "on close(normal, local)" event
// will be triggered. There is no immediate way to check whether
// the connection is closed, so a flag must be stored to remember
// the method was called.
//
// In the second case, an error occurred inside the read loop:
// - Upon receiving an error of type io.EOF the attendant will
//   be marked as closed, will not tell the connection to close,
//   and will trigger the "on close(normal, remote)" event.
// - Other errors are considered abnormal and will either be
//   connection errors or format errors. These errors will be
//   attended by telling the connection to close (ignoring any
//   error arising from that operation), and triggering the
//   event: "on close(error, <error instance>)".
// Any kind of these received errors will break the read loop,
// and the attendant will be useless and securely disposable.
//
// An internal state of the attendant will also be kept.
// Also, each attendant will hold its own data (e.g. current
// session ID), which will entirely depend on the application.
//
// Attendants can be used both in servers and clients (in these
// cases, they become particularly useful when there are many
// clients connecting to many different servers).
type Attendant struct {
	// The connection and the wrapper are the main elements
	// involved in the process. Although the wrapper will be
	// the object being used the most to send/receive data,
	// the connection is still needed to close it on need.
	connection     *net.TCPConn
	wrapper        MessageMarshaler
	// An internal status will also be needed, to track what
	// happens in the read loop and to trigger the proper
	// close event.
	status         AttendantStatus
	// Now, all the involved events.
	messageEvent   chan MessageEvent
	startedEvent   chan AttendantStartedEvent
	stoppedEvent   chan AttendantStoppedEvent
	// Arbitrary context which will be user-specific or
	// library-specific.
	context        map[string]interface{}
	// Throttling involves a mean to have dead time in which
	// the read loop does not process any message. Those dead
	// times occur after the last processed message, and they
	// don't use to be greater than 1 second in most applications
	// or games (although certain particular requests may have
	// different throttling times, a "general" throttling time
	// should seldom be > 1s). If using a throttle interval of
	// 0, no throttle will occur at all. The throttle interval
	// may be changed later.
	throttle       time.Duration
	throttleFrom   time.Time
	throttledEvent chan ThrottledEvent
}


// Starts the attendant (starts its read loop), after preparing
// the status and also triggering the onStart event appropriately.
func (attendant *Attendant) Start() error {
	if attendant.status == AttendantNew {
		go attendant.readLoop()
		return nil
	} else {
		return AttendantIsNotNew(true)
	}
}


// Closes the attendant (it will also end its read loop), also
// sets the end state and triggers the close event.
func (attendant *Attendant) Stop() error {
	if attendant.status != AttendantStopped {
		// noinspection GoUnhandledErrorResult
		attendant.connection.Close()
		return nil
	} else {
		return AttendantIsAlreadyStopped(true)
	}
}


// Returns a read-only channel with all the received messages.
func (attendant *Attendant) MessageEvent() <-chan MessageEvent {
	return attendant.messageEvent
}


// Returns a read-only channel with all the "attendant started" events.
func (attendant *Attendant) StartedEvent() <-chan AttendantStartedEvent {
	return attendant.startedEvent
}


// Returns a read-only channel with all the "throttled" events.
func (attendant *Attendant) ThrottledEvent() <-chan ThrottledEvent {
	return attendant.throttledEvent
}


// Returns a read-only channel with all the "attendant started" events.
func (attendant *Attendant) StoppedEvent() <-chan AttendantStoppedEvent {
	return attendant.stoppedEvent
}


// Writes a message via the connection, if it is not closed.
func (attendant *Attendant) Send(command string, args Args, kwargs KWArgs) error {
	if attendant.status != AttendantStopped {
		return attendant.wrapper.Send(command, args, kwargs)
	} else {
		return AttendantIsStopped(true)
	}
}


// Gets a context element by its key. Purely user-specific or
// library-specific.
func (attendant *Attendant) Context(key string) (interface{}, bool) {
	result, ok := attendant.context[key]
	return result, ok
}


// Sets a context element by its key. Purely user-specific or
// library-specific.
func (attendant *Attendant) SetContext(key string, value interface{}) {
	attendant.context[key] = value
}


// Removes a context element by its key. Purely user-specific or
// library-specific.
func (attendant *Attendant) RemoveContext(key string) {
	delete(attendant.context, key)
}


// Gets the throttle time for the current attendant.
func (attendant *Attendant) Throttle() time.Duration {
	return attendant.throttle
}


// Sets the throttle time for the current attendant.
// Negative throttle times will be negated, to positive.
func (attendant *Attendant) SetThrottle(throttle time.Duration) {
	if throttle < 0 {
		throttle = -throttle
	}
	attendant.throttle = throttle
}


func isClosedSocketError(err error) bool {
	if opError, ok := err.(*net.OpError); !ok {
		return false
	} else {
		// Notes: this error is literally the polls.ErrNetClosing error,
		// but it is illegal to import internals/poll.
		err = opError.Err
		return err == ErrNetClosing()
	}
}


// The read loop will attempt reading all the available data until
// it finds a gracefully-closed error, an extraneous error, or it
// was told to close beforehand. Received messages will be conveyed
// via some kind of central message channel.
func (attendant *Attendant) readLoop() {
	if attendant.status != AttendantNew {
		return
	}

	// First, the start event
	attendant.status = AttendantRunning
	attendant.startedEvent <- AttendantStartedEvent{attendant}

	// The stop type for the last event.
	var stopType AttendantStopType
	var stopError error

	Loop: for {
		if message, err, graceful := attendant.wrapper.Receive(); err != nil {
			if isClosedSocketError(err) {
				// The socket is closed. That happened
				// on our side.
				stopType = AttendantLocalStop
				break Loop
			} else if graceful {
				// This error is a graceful close.
				stopType = AttendantRemoteStop
				break Loop
			} else {
				// This error is not a graceful close.
				// It may be a non-graceful close or a decoding error.
				// net.Error objects are usually non-graceful errors.
				stopType = AttendantAbnormalStop
				stopError = err
				break Loop
			}
		} else {
			// The message arrived successfully, but the throttle must be
			// checked now to tell whether the messageEvent must pass the new
			// message, or not.
			if attendant.throttle == 0 {
				// No throttle is being used right now. It counts as "ok".
				// Moves the message to the messageEvent channel.
				attendant.messageEvent <- MessageEvent{attendant, message}
			} else if attendant.throttleFrom == (time.Time{}) {
				// Throttle is being used, but this is the first message
				// being received (no throttle can occur for it). It counts
				// as "ok" but the current time will be stored for the next
				// throttle.
				attendant.throttleFrom = time.Now()
				attendant.messageEvent <- MessageEvent{attendant, message}
			} else {
				// Now a throttle check starts. This means that if the lapse
				// between the current time and the previous message time is
				// greater than or equal to the throttle time, it counts as
				// "ok" but the current time will be stored for the next
				// throttle check. Otherwise, the message is throttled and
				// not processed.
				now := time.Now()
				lapse := now.Sub(attendant.throttleFrom)
				if lapse >= attendant.throttle {
					attendant.throttleFrom = now
					attendant.messageEvent <- MessageEvent{attendant, message}
				} else {
					attendant.throttledEvent <- ThrottledEvent{attendant, message, now, lapse}
				}
			}
		}
	}

	attendant.status = AttendantStopped
	if stopType != AttendantLocalStop {
		// noinspection GoUnhandledErrorResult
		attendant.connection.Close()
	}
	attendant.stoppedEvent <- AttendantStoppedEvent{attendant, stopType, stopError}
}


// Creates a new attendant, ready to be used.
func NewAttendant(connection *net.TCPConn, factory MessageMarshaler, throttle time.Duration,
	              startedEvent chan AttendantStartedEvent, stoppedEvent chan AttendantStoppedEvent,
	              messageEvent chan MessageEvent, throttledEvent chan ThrottledEvent) *Attendant {
	if throttle < 0 {
		throttle = -throttle
	}
	return &Attendant{
		connection:     connection,
		wrapper:        factory.Create(connection),
		status:         AttendantNew,
		messageEvent:   messageEvent,
		startedEvent:   startedEvent,
		stoppedEvent:   stoppedEvent,
		context:        make(map[string]interface{}),
		throttle:       throttle,
		throttledEvent: throttledEvent,
	}
}


// Creates an autonomous client (in a context where only one is needed).
func NewClient(connection *net.TCPConn, factory MessageMarshaler, throttle time.Duration, bufferSize uint) *Attendant {
	return NewAttendant(
		connection, factory, throttle, make(chan AttendantStartedEvent), make(chan AttendantStoppedEvent),
		make(chan MessageEvent, bufferSize), make(chan ThrottledEvent, bufferSize),
	)
}