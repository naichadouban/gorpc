package gorpc

// AuthenticateCmd defines the authenticate JSON-RPC command.
type AuthenticateCmd struct {
	Username   string
	Passphrase string
}
