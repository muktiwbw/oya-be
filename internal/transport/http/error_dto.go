package httpapi

type ErrorResponse struct {
	Error ErrorDTO `json:"error"`
}

type ErrorDTO struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Details []ErrorDetailDTO `json:"details,omitempty"`
}

type ErrorDetailDTO struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
