package handler

import (
	"api-gateway/internal/handler/response"
	"io"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
	"go.uber.org/zap"
)

// Delete 删除片段
func (h *NoteHandler) Delete(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	snippetID := c.Param("id")
	if snippetID == "" { response.BadRequest(c, "笔记ID不能为空"); return }

	res, err := h.noteClient.DeleteSnippet(c.Request.Context(), userID, snippetID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("删除笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Search 搜索片段
func (h *NoteHandler) Search(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	
	rawQuery := c.Request.URL.RawQuery
	res, err := h.noteClient.SearchSnippets(c.Request.Context(), userID, rawQuery)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("搜索笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetPublic 获取公开片段 (不需要登录)
func (h *NoteHandler) GetPublic(c *gin.Context) {
	snippetID := c.Param("id")
	if snippetID == "" { response.BadRequest(c, "笔记ID不能为空"); return }

	res, err := h.noteClient.GetPublicSnippet(c.Request.Context(), snippetID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取公开笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Favorite 收藏片段
func (h *NoteHandler) Favorite(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	snippetID := c.Param("id")

	res, err := h.noteClient.FavoriteSnippet(c.Request.Context(), userID, snippetID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("收藏笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Unfavorite 取消收藏片段
func (h *NoteHandler) Unfavorite(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	snippetID := c.Param("id")

	res, err := h.noteClient.UnfavoriteSnippet(c.Request.Context(), userID, snippetID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("取消收藏笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// CreateFromTemplate 基于模板创建
func (h *NoteHandler) CreateFromTemplate(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "请求体格式错误")
		return
	}

	res, err := h.noteClient.CreateSnippetFromTemplate(c.Request.Context(), userID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("使用模板创建笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// ListShared 与我共享列表
func (h *NoteHandler) ListShared(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetSharedSnippets(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取共享笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// ListFavorites 收藏列表
func (h *NoteHandler) ListFavorites(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetFavoriteSnippets(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取收藏笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetGroups 分组列表
func (h *NoteHandler) GetGroups(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetGroups(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取分组列表失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) CreateGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var payload map[string]any
	c.ShouldBindJSON(&payload)
	res, err := h.noteClient.CreateGroup(c.Request.Context(), userID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("创建分组失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) UpdateGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var payload map[string]any
	c.ShouldBindJSON(&payload)
	groupID := c.Param("id")
	res, err := h.noteClient.UpdateGroup(c.Request.Context(), userID, groupID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("更新分组失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) DeleteGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	groupID := c.Param("id")
	res, err := h.noteClient.DeleteGroup(c.Request.Context(), userID, groupID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("删除分组失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetTags 标签列表
func (h *NoteHandler) GetTags(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetTags(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取标签列表失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) CreateTag(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var payload map[string]any
	c.ShouldBindJSON(&payload)
	res, err := h.noteClient.CreateTag(c.Request.Context(), userID, payload)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("创建标签失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) DeleteTag(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	tagID := c.Param("id")
	res, err := h.noteClient.DeleteTag(c.Request.Context(), userID, tagID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("删除标签失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// GetTemplates 模板列表
func (h *NoteHandler) GetTemplates(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetTemplates(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取模板列表失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

func (h *NoteHandler) GetTemplate(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	templateID := c.Param("id")
	res, err := h.noteClient.GetTemplate(c.Request.Context(), userID, templateID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取模板失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Upload 文件上传
func (h *NoteHandler) Upload(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	
	contentType := c.Request.Header.Get("Content-Type")
	res, err := h.noteClient.UploadFile(c.Request.Context(), userID, contentType, c.Request.Body)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("上传文件失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	c.Request.Body = io.NopCloser(c.Request.Body) // 保护原body
	response.Success(c, res)
}

func (h *NoteHandler) ListRecent(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	res, err := h.noteClient.GetRecentSnippets(c.Request.Context(), userID)
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("获取最近访问笔记失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}
