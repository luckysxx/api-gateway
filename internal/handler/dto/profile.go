package dto

type GetProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	UpdatedAt string `json:"updated_at"`
}

type UpdateProfileRequest struct {
	Nickname  string `json:"nickname" binding:"omitempty,max=32"`
	AvatarURL string `json:"avatar_url" binding:"omitempty,max=512"`
	Bio       string `json:"bio" binding:"omitempty,max=256"`
}

// UpdateProfileResponse usually just returns the updated profile
type UpdateProfileResponse struct {
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	UpdatedAt string `json:"updated_at"`
}
