package gorpc

// ErrClientQuit describes the error where a client send is not processed due
// to the client having already been disconnected or dropped.
var ErrClientQuit = errors.New("client quit")
