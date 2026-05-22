package model

// ModelCapability describes which endpoints a model supports,
// whether it belongs to the conversation family, and whether
// it is currently available (has at least one enabled channel).
type ModelCapability struct {
	Name         string   `json:"name"`
	Endpoints    []string `json:"endpoints"`
	Conversation bool     `json:"conversation"`
	Available    bool     `json:"available"`
}
