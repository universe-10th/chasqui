package chasqui

import (
	"io"
	"net"
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


// Messages being conveyed come in another kind of structure: The
// structure will hold both the conveyed message and the attendant
// that received and conveyed it.
type Conveyed struct {
	Attendant *Attendant
	Message   *Message
}


// Callback to report when an attendant successfully ran
// its lifecycle.
type OnAttendantStart func(*Attendant)


// Callback to report when an attendant terminated its
// lifecycle, either by local or remote command, or by an
// error that was triggered.
type OnAttendantStop func(*Attendant, AttendantStopType, error)


// Attendants are spawned objects and routines for a single
// incoming connection. They are created using certain protocol
// factory (an instance of MessageMarshaler), are connected to
// a "central conveyor" (one for all the bunch of messages) to
// read the incoming messages via an individual goroutine that
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
// - Upon receiving an error of type io.ErrClosedPipe, the
//   same will apply.
// - Other errors are considered abnormal and will either be
//   connection errors or format errors. These errors will be
//   attended by telling the connection to close (ignoring any
//   error arising from that operation), and triggering the
//   event: "on close(error, <error instance>)".
// Any kind of these received errors will break the read loop,
// and the attendant will be useless and securely disposable.
//
// An internal state of the attendant will also be kept.
type Attendant struct {
	// The connection and the wrapper are the main elements
	// involved in the process. Although the wrapper will be
	// the object being used the most to send/receive data,
	// the connection is still needed to close it on need.
	connection *net.TCPConn
	wrapper    MessageMarshaler
	// An internal status will also be needed, to track what
	// happens in the read loop and to trigger the proper
	// close event.
	status     AttendantStatus
	// Now, the two needed events and the "conveyor".
	conveyor   chan Conveyed
	onStart    OnAttendantStart
	onStop     OnAttendantStop
}


// Starts the attendant (starts its read loop), after preparing
// the status and also triggering the onStart event appropriately.
func (attendant *Attendant) Start() error {
	if attendant.status == AttendantNew {
		attendant.status = AttendantRunning
		attendant.onStart(attendant)
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
		attendant.onStop(attendant, AttendantLocalStop, nil)
		_ = attendant.connection.Close()
		attendant.status = AttendantStopped
		return nil
	} else {
		return AttendantIsAlreadyStopped(true)
	}
}


// Writes a message via the connection, if it is not closed.
func (attendant *Attendant) Send(message *Message) error {
	if attendant.status != AttendantStopped {
		return attendant.wrapper.Send(message)
	} else {
		return AttendantIsStopped(true)
	}
}


// The read loop will attempt reading all the available data until
// it finds a gracefully-closed error, an extraneous error, or it
// was told to close beforehand. Received messages will be conveyed
// via some kind of "central conveyor" channel.
func (attendant *Attendant) readLoop() {
	for {
		if message, err := attendant.wrapper.Receive(); err != nil {
			if attendant.status == AttendantRunning {
				if err == io.EOF || err == io.ErrClosedPipe {
					// This error is a graceful close, or a rejection to
					// start a read operation because the underlying socket
					// is already closed and the decoder implementation uses
					// the io.ErrClosedPipe for those cases.
					attendant.onStop(attendant, AttendantRemoteStop, nil)
				} else {
					// This error is not a graceful close.
					// It may be a non-graceful close or a decoding error.
					// net.Error objects are usually non-graceful errors.
					attendant.onStop(attendant, AttendantAbnormalStop, nil)
					_ = attendant.connection.Close()
				}
				attendant.status = AttendantStopped
			}
			return
		} else {
			// Moves the message to the conveyor channel.
			attendant.conveyor <- Conveyed{attendant, message}
		}
	}
}


// Creates a new attendant, ready to be used.
func NewAttendant(connection *net.TCPConn, factory MessageMarshaler, conveyor chan Conveyed,
	              onStart OnAttendantStart, onStop OnAttendantStop) *Attendant {
	return &Attendant{
		connection: connection,
		wrapper: factory.Create(connection),
		status: AttendantNew,
		conveyor: conveyor,
		onStart: onStart,
		onStop: onStop,
	}
}