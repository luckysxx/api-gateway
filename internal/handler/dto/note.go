package dto

// CreateSnippetRequest 创建笔记请求体
type CreateSnippetRequest struct {
	Title      string `json:"title" binding:"required,max=128"`
	Content    string `json:"content" binding:"required"`
	Language   string `json:"language" binding:"required,max=32"`
	Visibility string `json:"visibility" binding:"omitempty,oneof=public private"`
}

// UpdateSnippetRequest 更新笔记请求体
type UpdateSnippetRequest struct {
	Title      string `json:"title" binding:"required,max=128"`
	Content    string `json:"content" binding:"required"`
	Language   string `json:"language" binding:"required,max=32"`
	Visibility string `json:"visibility" binding:"omitempty,oneof=public private"`
}

// SnippetResponse 笔记统一响应体
type SnippetResponse struct {
	ID         string `json:"id"`
	OwnerID    string `json:"owner_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Language   string `json:"language"`
	Visibility string `json:"visibility"`
	Type       string `json:"type,omitempty"`
	FileURL    string `json:"file_url,omitempty"`
	FileSize   string `json:"file_size,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	GroupID    string `json:"group_id,omitempty"`
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
