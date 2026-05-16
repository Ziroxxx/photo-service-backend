package user

type RegisterRequest struct {
	Login    string `json:"login" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	FullName string `json:"fullName" binding:"required,min=2,max=255"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type PatchRoleRequest struct {
	Role Role `json:"role" binding:"required,oneof=admin organizer photographer participant"`
}

type PatchStatusRequest struct {
	Status Status `json:"status" binding:"required,oneof=active blocked pending"`
}

type AuthResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         User   `json:"user"`
}
