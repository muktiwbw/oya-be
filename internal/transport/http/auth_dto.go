package httpapi

type UserDTO struct {
	PublicID    string `json:"public_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type AuthResponseData struct {
	Token     string  `json:"token"`
	TokenType string  `json:"token_type"`
	User      UserDTO `json:"user"`
}

type AuthResponse struct {
	Data AuthResponseData `json:"data"`
}
type UserResponse struct {
	Data UserDTO `json:"data"`
}
