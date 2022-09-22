package client

type ClientConfig struct {
	RepresentationPercent  float64            `json:"representation_percent"`
	ClientType             string             `json:"client_type"`
	HumanizedLabel         string             `json:"humanized_label"`
	ObeysServerPollAfter   bool               `json:"obeys_server_poll_after"`
	MaxInitialDelayMs      int                `json:"max_initial_delay_ms"`
	MaxNetworkJitterMs     int                `json:"max_network_jitter_ms"`
	CustomStringProperties map[string]string  `json:"custom_string_properties"`
	CustomIntProperties    map[string]int     `json:"custom_int_properties"`
	CustomFloatProperties  map[string]float64 `json:"custom_float_properties"`
	CustomBoolProperties   map[string]bool    `json:"custom_bool_properties"`
}
