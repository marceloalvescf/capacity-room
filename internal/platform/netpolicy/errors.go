package netpolicy

// Code is a refusal of the policy. It is the whole of what an Error says.
type Code string

// The refusals of the policy.
const (
	CodeURLRejected        Code = "ENDPOINT_URL_REJECTED"
	CodeAddressRejected    Code = "ENDPOINT_ADDRESS_REJECTED"
	CodeResolutionFailed   Code = "ENDPOINT_RESOLUTION_FAILED"
	CodeConnectionRejected Code = "ENDPOINT_CONNECTION_REJECTED"
	CodeRedirectRejected   Code = "ENDPOINT_REDIRECT_REJECTED"
)

// Error carries a code and nothing else: no URL, host, or address.
type Error struct{ Code Code }

// Error returns the code as text.
func (e *Error) Error() string { return string(e.Code) }

// PolicyRefusal marks the error as a refusal of the policy, which a client
// returns as it is instead of treating it as a failure to connect.
func (e *Error) PolicyRefusal() {}

func refuse(code Code) error { return &Error{Code: code} }
