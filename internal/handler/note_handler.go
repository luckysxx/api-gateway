package handler

import (
	"api-gateway/internal/handler/dto"
	"api-gateway/internal/handler/response"
	"api-gateway/internal/handler/validator"
	"api-gateway/internal/restclient"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
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
// GET /api/v1/notes/me/snippets → go-note GET /api/v1/me/snippets
func (h *NoteHandler) ListMine(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	snippets, err := h.noteClient.ListMySnippets(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取用户笔记列表失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, err)
		return
	}

	response.Success(c, snippets)
}

// Create 创建笔记
// POST /api/v1/notes/snippets → go-note POST /api/v1/snippets
func (h *NoteHandler) Create(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	var req dto.CreateSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	payload := map[string]any{
		"title":      req.Title,
		"content":    req.Content,
		"language":   req.Language,
		"visibility": req.Visibility,
	}

	snippet, err := h.noteClient.CreateSnippet(c.Request.Context(), userID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("创建笔记失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, err)
		return
	}

	response.Success(c, snippet)
}

// Get 获取单条笔记详情
// GET /api/v1/notes/snippets/:id → go-note GET /api/v1/snippets/:id
func (h *NoteHandler) Get(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	snippetID := c.Param("id")
	if snippetID == "" {
		response.BadRequest(c, "笔记ID不能为空")
		return
	}

	snippet, err := h.noteClient.GetSnippet(c.Request.Context(), userID, snippetID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取笔记详情失败",
			zap.Int64("userID", userID),
			zap.String("snippetID", snippetID),
			zap.Error(err),
		)
		response.Error(c, err)
		return
	}

	response.Success(c, snippet)
}

// Update 更新笔记
// PUT /api/v1/notes/snippets/:id → go-note PUT /api/v1/snippets/:id
func (h *NoteHandler) Update(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok {
		return
	}

	snippetID := c.Param("id")
	if snippetID == "" {
		response.BadRequest(c, "笔记ID不能为空")
		return
	}

	var req dto.UpdateSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	payload := map[string]any{
		"title":      req.Title,
		"content":    req.Content,
		"language":   req.Language,
		"visibility": req.Visibility,
	}

	snippet, err := h.noteClient.UpdateSnippet(c.Request.Context(), userID, snippetID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("更新笔记失败",
			zap.Int64("userID", userID),
			zap.String("snippetID", snippetID),
			zap.Error(err),
		)
		response.Error(c, err)
		return
	}

	response.Success(c, snippet)
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
