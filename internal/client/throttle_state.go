package client

// Checkout throttle state machine.

// ThrottleState represents the state of the client. If the client has not
// clicked on the checkout button, there won't be a state at all. There
// is a total of 4 states: (empty), queued, inCheckout, and exited.
//
// (empty)    = before clicking the checkout button
// queued     = queued to checkout backlog by the system (~ nginx 'throttled')
// inCheckout = successfully entered the checkout area ( ~ nginx 'passed' )
// exited     = exited flow (whatever the reason --> completion included)
//
// (empty) ---> queued -----> inCheckout (if poll succeeds) -----> exited
//   |
//   |
//   -------> inCheckout -----> exited
//
const (
	ThrottleStateKey = "ThrottleState"
)

type ThrottleState int

const (
	Initial ThrottleState = iota
	Queued
	InCheckout
	Exited
)

func (s ThrottleState) String() string {
	return [...]string{"initial", "queued", "inCheckout", "exited"}[s]
}
