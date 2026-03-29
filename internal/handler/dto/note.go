package dto

// CreatePasteRequest 创建笔记请求体
type CreatePasteRequest struct {
	Title      string `json:"title" binding:"required,max=128"`
	Content    string `json:"content" binding:"required"`
	Language   string `json:"language" binding:"required,max=32"`
	Visibility string `json:"visibility" binding:"omitempty,oneof=public private"`
}

// UpdatePasteRequest 更新笔记请求体
type UpdatePasteRequest struct {
	Title      string `json:"title" binding:"required,max=128"`
	Content    string `json:"content" binding:"required"`
	Language   string `json:"language" binding:"required,max=32"`
	Visibility string `json:"visibility" binding:"omitempty,oneof=public private"`
}

// PasteResponse 笔记统一响应体
type PasteResponse struct {
	ID         int64  `json:"id"`
	OwnerID    int64  `json:"owner_id"`
	Title      string `json:"title"`
	ShortLink  string `json:"short_link,omitempty"`
	Content    string `json:"content"`
	Language   string `json:"language"`
	Visibility string `json:"visibility"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// RefreshTokenRequest Token 刷新请求体
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse Token 刷新响应体
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
