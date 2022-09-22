package network_mock

import "time"

type SessionData struct {
	Id               int
	QueueEntryTime   time.Time
	AdvisedPollAfter time.Time
	ThrottleState    string
	ThrottleCookie   map[string]string
}
