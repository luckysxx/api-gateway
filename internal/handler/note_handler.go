package handler

import (
	"api-gateway/internal/handler/dto"
	"api-gateway/internal/handler/response"
	"api-gateway/internal/handler/validator"
	"api-gateway/internal/restclient"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NoteHandler 处理笔记模块的 BFF 路由，将前端请求转换为对 go-note 微服务的 REST 调用。
type NoteHandler struct {
	noteClient restclient.NoteClient
	log        *zap.Logger
}

// NewNoteHandler 创建 NoteHandler 实例。
func NewNoteHandler(noteClient restclient.NoteClient, log *zap.Logger) *NoteHandler {
	return &NoteHandler{noteClient: noteClient, log: log}
}

// ListMine 获取当前登录用户的笔记列表
// GET /api/v1/notes/me/pastes → go-note GET /api/v1/me/pastes
func (h *NoteHandler) ListMine(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	pastes, err := h.noteClient.ListMyPastes(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("获取用户笔记列表失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, err)
		return
	}

	response.Success(c, pastes)
}

// Create 创建笔记
// POST /api/v1/notes/pastes → go-note POST /api/v1/pastes
func (h *NoteHandler) Create(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	var req dto.CreatePasteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		h.log.Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	payload := map[string]any{
		"title":      req.Title,
		"content":    req.Content,
		"language":   req.Language,
		"visibility": req.Visibility,
	}

	paste, err := h.noteClient.CreatePaste(c.Request.Context(), userID, payload)
	if err != nil {
		h.log.Error("创建笔记失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, err)
		return
	}

	response.Success(c, paste)
}

// Get 获取单条笔记详情
// GET /api/v1/notes/pastes/:id → go-note GET /api/v1/pastes/:id
func (h *NoteHandler) Get(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	pasteID := c.Param("id")
	if pasteID == "" {
		response.BadRequest(c, "笔记ID不能为空")
		return
	}

	paste, err := h.noteClient.GetPaste(c.Request.Context(), userID, pasteID)
	if err != nil {
		h.log.Error("获取笔记详情失败",
			zap.Int64("userID", userID),
			zap.String("pasteID", pasteID),
			zap.Error(err),
		)
		response.Error(c, err)
		return
	}

	response.Success(c, paste)
}

// Update 更新笔记
// PUT /api/v1/notes/pastes/:id → go-note PUT /api/v1/pastes/:id
func (h *NoteHandler) Update(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	pasteID := c.Param("id")
	if pasteID == "" {
		response.BadRequest(c, "笔记ID不能为空")
		return
	}

	var req dto.UpdatePasteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		h.log.Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	payload := map[string]any{
		"title":      req.Title,
		"content":    req.Content,
		"language":   req.Language,
		"visibility": req.Visibility,
	}

	paste, err := h.noteClient.UpdatePaste(c.Request.Context(), userID, pasteID, payload)
	if err != nil {
		h.log.Error("更新笔记失败",
			zap.Int64("userID", userID),
			zap.String("pasteID", pasteID),
			zap.Error(err),
		)
		response.Error(c, err)
		return
	}

	response.Success(c, paste)
}

// extractUserID 从 JWT 中间件注入的上下文中提取用户 ID，统一鉴权失败的响应逻辑。
func (h *NoteHandler) extractUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return 0, false
	}
	return val.(int64), true
}
