# CHASQUI
CHAnnel / Socket QUeuing Interface allows us to create a standard TCP socket server and manage incoming messages and
connections with a queuing system.

Requirements
------------

This package was tested to work with go 1.14. No extra requirements are needed.

Usage (basic)
-------------

First, a server must be created. This package comes with a default implementation which can be used to create a TCP
server:

1. Choose a `MessageMarshaler` type. This package comes with `marshalers/json/JSONMessageMarshaler` by default, but any
   type could be used/created.
2. Create the instance of the server (in this caase, the JSON example will be used).

   ```
   import (
       "github.com/universe-10th/chasqui"
       "github.com/universe-10th/chasqui/marshalers/json"
   )
   
   var server = chasqui.NewServer(
       &json.JSONMessageMarshaler{}, 1024, 1, 0,
   )
   ```
    
   Now the server is created, it must start in certain binding. Any standard TCP (v4 or v6) binding will do the trick.
    
   ```
   if err := server.Run("0.0.0.0:3000"); err != nil {
       fmt.Printf("An error was raised while trying to start the server at address 0.0.0.0:3000: %s\n", err)
       return
   }
   ```
    
   Once the server is running, a lifecycle must be defined for the serve. Such lifecycle must be a loop consuming all
   the available channels in the server. It must have this structure:
    
   ```
   func lifecycle(basicServer *chasqui.BasicServer) {
       Loop: for {
           select {
           case event := <-basicServer.StartedEvent():
               // The server has just started.
               // event.Addr: The *net.TCPAddr this server was bound to.
           case event := <-basicServer.AcceptFailedEvent():
               // An error was encountered while trying to accept a connection.
               // The event itself is the error.
           case <-basicServer.StoppedEvent():
               // The server has been stopped locally.
               // There are no fields here.
           case event := <-basicServer.AttendantStartedEvent():
               // A socket has just been accepted (for client sockets: the socket has just started its lifecycle).
           case event := <-basicServer.MessageEvent():
               // A message has just arrived.
               // event.Attendant: The socket receiving the message.
               // event.Message: The message.
           case event := <-basicServer.ThrottledEvent():
               // A message was throttled. This, because the throttle was set for a particular socket to a nonzero
               // value.
               // event.Attendant: The socket reporting the throttle.
               // event.Message: The throttled message.
               // event.Instant: The exact instant of the message being received.
               // event.Lapse: The lapse that was not accepted since the last message (and thus throttled).
           case event := <-basicServer.AttendantStoppedEvent():
               // A socket was disconnected.
               // event.Attendant: The socket being disconnected.
               // event.StopType: The stop type.
               // - AttendantLocalStop: The socket was stopped locally.
               // - AttendantRemoteStop: The socket was stopped remotely (gracefully).
               // - AttendantAbnormalStop: The socket was stopped abnormally (due to a strange socket error, or an
               //   encoding/decoding error).
               // event.Error: For the AttendantAbnormalStop stop type, it will report the underlying error.
           }
       }
   }
   ```
   
   and be invoked: `go lifecycle(myServer)`. It is up to the user to define any logic, but all the channels should be
   included in this for/select structure to avoid channels being blocked by not being consumed.
   
   Alternatively, a convenience function may be invoked: `chasqui.ServerFunnel(myServer, myHandler)` where `myHandler`
   implements `chasqui.BasicServerFunnel`:
   
   ```
   type BasicServerFunnel interface {
       Started(*BasicServer, *net.TCPAddr)
       AcceptFailed(*BasicServer, error)
       Stopped(*BasicServer)
       AttendantStarted(*BasicServer, *Attendant)
       MessageArrived(*BasicServer, *Attendant, Message)
       MessageThrottled(*BasicServer, *Attendant, Message, time.Time, time.Duration)
       AttendantStopped(*BasicServer, *Attendant, AttendantStopType, error)
   }
   ```
   
   Invoking `chasqui.ServerFunnel` will spawn a goroutine quite similar to the `go lifecycle(server)` example, but in
   this case all the channels are guaranteed to be consumed, and for each consumption a respective callback method will
   be invoked.
   
   Now the server is running. The sockets are instances of `*chasqui.Attendant` and, as long as they are not closed,
   they can be easily used to send messages or keep context data (think of current session data, which is particular to
   a socket).
    
3. Using a socket to send a message:

   ```
   import (
       "github.com/universe-10th/chasqui/types"
   )
   
   anAttendant.Send("Hello", types.Args{1, false, "foo"}, types.KWArgs{"foo": "bar", "baz": 1})
   ```
   
   Three arguments are specified as a string, a `types.Args` which is actually `[]interface{}` and can be `nil`, and a
   `types.KWArgs` which is actually `map[string]interface{}` and can also be nil.
   
   Under the hoods, that call will build a message and send it. A message is an implementation of the `types.Message`
   interface, which defines three methods:
   
   - `Command() string`: The message.
   - `Args() types.Args`: The optional sequential arguments.
   - `KWArgs() types.KWArgs`: The optional named arguments.

4. Managing the attendant's context:

   - `value, exists := attendant.Context(key)`: Works like it would by subscripting a `map[string]interface{}`.
   - `attendant.SetContext("foo", anyValue)`: Sets a value to the current socket data.
   - `attendant.RemoveContext("foo")`: Removes a value being previously set in the current socket data.

5. Changing the attendant's throttle:

   - `attendant.SetThrottle(lapse time.Duration)`: Sets the expected minimum interval between incoming messages
     to the given duration. Use a duration of 0 to disable it. Using negative values is the same as their positive
     counterparts.
   - `throttle := attendant.Throttle()`: Gets the attendant's current throttle.

Usage (Custom)
--------------

Custom servers can be created without using the Basic Server components and funnels.

For this to work, invoke `chasqui.Dispatcher` and provide all the callbacks for the different events:

   - `onStart = func(*Dispatcher, *net.TCPAddr) { ... }`
   - `onAcceptSuccess = func(*Dispatcher, *net.TCPConn) { ... }`
   - `onAcceptError = func(*Dispatcher, error) { ... }`
   - `onStop = func(*Dispatcher) { ... }`

Usually, the `onAcceptSuccess` callback involves instantiating an attendant using a call like this:

   ```
   attendant := chasqui.NewAttendant(connection, factory, throttle, startedEvent, stoppedEvent,
                                     messageEvent, throttledEvent)
   attendant.Start()
   ```

Where the first argument is the accepted connection, the second argument is the throttle as a time.Duration (use 0 to
disable it), and custom channels to receive start, stop, message received, and message throttled events. It is up to
the implementor to keep, track, and remove the sockets along their lifecycle. Even funneling features have to be
manually implemented.

Clients
-------

Client sockets can also be created. To make this work, make a standard connection with `net.TCPDial` and wrap that
connection with `NewAttendant`, as in the case for custom dispatchers. Then, `client.Start` must be called, and also
the channels must be provided.

Alternatively, `chasqui.NewBasicClient` wraps the connection and also creates the channels.

Then, a lifecycle goroutine can be defined around the just-created attendant, or a similar funneling approach, ivolving
implementing this interface:

   ```
   type BasicClientFunnel interface {
	   Started(*Attendant)
	   MessageArrived(*Attendant, Message)
	   MessageThrottled(*Attendant, Message, time.Time, time.Duration)
	   Stopped(*Attendant, AttendantStopType, error)
   }
   ```

and then instantiating one and invoking (before `.Start()`-ing):

   ```
   chasqui.ClientFunnel(myAttendant, myFunnel)
   ```

Custom marshalers
-----------------

They are created by implementing the `types.MessageMarshaler` interface, defined like this:

```
type MessageMarshaler interface {
    Receive()                                      (Message, error, bool)
    Send(command string, args Args, kwargs KWArgs) error
    // Constructor - Creates a new marshaler by its buffer.
    Create(io.ReadWriter)                          MessageMarshaler
}
```

Considering:

- The return values in `Receive()` stand for the Message, the error (which could be a socket error or a
  per-implementation decoding error), and whether the error should be considered a "graceful close" error
  or an abnormal one (e.g. a format error). `Receive()` should be blocking until a message arrives or an
  error occurs, and the underlying implementation should slide through its buffer (socket) and not do any
  other change on it.
- `Send(...)` should take those arguments, serialize them, and send them through the socket. Sending a
  message should, in the end, write to the buffer without doing anything else.
- `Create(...)` should take an `io.ReadWriter` and return a __new__ instance. It is intended to be invoked
  like this: `marshaler := &YourClass{}.Create(aSocket)`.