package json

import (
	"io"
	json2 "encoding/json"
	. "github.com/universe-10th/chasqui/types"
)


// The internal struture tu pass JSON objects.
type message struct {
	C   string
	A   Args
	KWA KWArgs
}


// Retrieves the command of this message, as
// per the interface implementation.
func (msg message) Command() string {
	return msg.C
}


// Retrieves the args of this message, as
// per the interface implementation.
func (msg message) Args() Args {
	return msg.A
}


// Retrieves the kwargs of this message, as
// per the interface implementation.
func (msg message) KWArgs() KWArgs {
	return msg.KWA
}


// Marshals JSON messages around a read-writer.
type JSONMessageMarshaler struct {
	encoder *json2.Encoder
	decoder *json2.Decoder
}


// Receives a JSON message from the underlying
// buffer (socket, most likely).
func (marshaler *JSONMessageMarshaler) Receive() (Message, error) {
	msg := &message{}
	if err := marshaler.decoder.Decode(&msg); err != nil {
		return nil, err
	} else {
		return msg, nil
	}
}


// Sends a JSON message via the underlying buffer
// (socket, most likely).
func (marshaler *JSONMessageMarshaler) Send(command string, args Args, kwargs KWArgs) error {
	return marshaler.encoder.Encode(message{command, args, kwargs})
}


// Creates a new instance of JSON marshaler around
// a buffer (socket, most likely).
func (marshaler *JSONMessageMarshaler) Create(buffer io.ReadWriter) MessageMarshaler {
	return &JSONMessageMarshaler{
		encoder: json2.NewEncoder(buffer),
		decoder: json2.NewDecoder(buffer),
	}
}