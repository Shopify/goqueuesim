package tracker

type Tracker interface {
	GetFeedbackChannel() chan Feedback
	MonitorAndEmitFeedback(lowUtilMaxThresholdPct float64, maxSkpdLowUtilCount int)
	ShouldProceed(clientId int) bool
}
