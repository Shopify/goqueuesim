package tracker

type Feedback struct {
	PollingUtil    float64
	CheckoutUtil   float64
	CustomFeedback map[string]interface{}
}
