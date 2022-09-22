package network_mock

type requestEndpointEnum int

const (
	CheckoutEndpoint requestEndpointEnum = iota
	PollingEndpoint
)

type MockRequest struct {
	Endpoint   requestEndpointEnum
	ClientData SessionData
}

func MakeCheckoutRequest(session SessionData) *MockRequest {
	return &MockRequest{Endpoint: CheckoutEndpoint, ClientData: session}
}

func MakePollRequest(session SessionData) *MockRequest {
	return &MockRequest{Endpoint: PollingEndpoint, ClientData: session}
}
