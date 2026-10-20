package httpapi

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserDTO struct {
	PublicID    string `json:"public_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type LoginResponseData struct {
	Token     string  `json:"token"`
	TokenType string  `json:"token_type"`
	User      UserDTO `json:"user"`
}

type LoginResponse struct {
	Data LoginResponseData `json:"data"`
}
