package types

import "io"


// Messages are flow bundles that will exist in
// either direction. Clients will usually use them
// to invoke an action or perform a request, while
// servers will use them to notify or respond to
// client requests.
//
// Since there will exist several different types
// of brokers (e.g. json, msgpack, ...) they will
// need their own structs and tags, but all of them
// will implement this interface.
type Message interface {
	Command() string
	Args()    []interface{}
	KWArgs()  map[string]interface{}
}


// Message Marshalers are wrappers around a read-write
// object, and will do their magic to receive / send
// Message objects (implementations will vary, but the
// interface will be respected). This interface has a
// constructor taking a read-writer and creating the
// wrapper for it.
type MessageMarshaler interface {
	Receive()                                                               (Message, error)
	Send(command string, args []interface{}, kwargs map[string]interface{}) error
	// Constructor - Creates a new marshaler by its buffer.
	Create(io.ReadWriter)                                                   MessageMarshaler
}
