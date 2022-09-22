package network_mock

type MockResponse struct {
	ClientData SessionData
}

func MakeServerResponse(session SessionData) *MockResponse {
	return &MockResponse{ClientData: session}
}
