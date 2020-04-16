package chasqui


// An error in an argument while instantiating anything in this library.
type ArgumentError struct {
	argument string
}


// Returns the argument name which caused the error.
func (argumentError ArgumentError) Argument() string {
	return argumentError.argument
}


// Returns the error message.
func (argumentError ArgumentError) Error() string {
	return "Argument error: " + argumentError.argument
}