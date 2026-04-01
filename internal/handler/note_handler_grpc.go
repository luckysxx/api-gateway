package handler

import (
	"api-gateway/internal/grpcclient"
	"api-gateway/internal/handler/dto"
	"api-gateway/internal/handler/response"
	commonlogger "github.com/luckysxx/common/logger"
	notepb "github.com/luckysxx/common/proto/note"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"fmt"
	"strconv"
)

// NoteHandlerGrpc 提供了通过 gRPC 访问 go-note 的网关代理层实现，
// 该结构体对标 NoteHandler，作为双轨制架构下的高性能内部通信替代方案。
type NoteHandlerGrpc struct {
	noteClient notepb.NoteServiceClient
	log        *zap.Logger
}

func emptyIfNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func toSnippetDTO(item *notepb.SnippetResponse) *dto.SnippetResponse {
	if item == nil {
		return nil
	}

	result := &dto.SnippetResponse{
		ID:         fmt.Sprintf("%d", item.Id),
		OwnerID:    fmt.Sprintf("%d", item.OwnerId),
		Title:      item.Title,
		Content:    item.Content,
		Language:   item.Language,
		Visibility: item.Visibility,
		Type:       item.Type,
		FileURL:    item.FileUrl,
		MimeType:   item.MimeType,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}

	if item.FileSize != 0 {
		result.FileSize = fmt.Sprintf("%d", item.FileSize)
	}
	if item.GroupId != 0 {
		result.GroupID = fmt.Sprintf("%d", item.GroupId)
	}

	return result
}

func toSnippetDTOList(items []*notepb.SnippetResponse) []dto.SnippetResponse {
	if items == nil {
		return []dto.SnippetResponse{}
	}

	result := make([]dto.SnippetResponse, 0, len(items))
	for _, item := range items {
		if converted := toSnippetDTO(item); converted != nil {
			result = append(result, *converted)
		}
	}
	return result
}

// NewNoteHandlerGrpc 构造函数：创建一个新的基于 gRPC 协议的笔记模块网关 Handler。
func NewNoteHandlerGrpc(noteClient notepb.NoteServiceClient, log *zap.Logger) *NoteHandlerGrpc {
	return &NoteHandlerGrpc{noteClient: noteClient, log: log}
}

// extractUserID 从网关前置 JWT 中间件注入的 Context 中提取并转换出经过鉴权的用户 ID。
func (h *NoteHandlerGrpc) extractUserID(c *gin.Context) (int64, bool) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return 0, false
	}
	return val.(int64), true
}

// =========================================================================
// 核心片段接管 (Core Snippet Router Handlers)
// =========================================================================

// ListMine 代理查询：请求用户的个人代码片段列表。
func (h *NoteHandlerGrpc) ListMine(c *gin.Context) {
	// TODO: 未完成 - 当前直接透传，未来可追加分页和排序筛选处理
	userID, ok := h.extractUserID(c)
	if !ok { return }

	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListSnippets(ctx, &notepb.ListSnippetsRequest{})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC ListMine 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTOList(res.Snippets))
}

// Create 代理创建：请求新建代码片段。
func (h *NoteHandlerGrpc) Create(c *gin.Context) {
	// TODO: 未完成 - API 层可追加针对 Title/Content 的敏感词过滤
	userID, ok := h.extractUserID(c)
	if !ok { return }

	var req notepb.CreateSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数验证失败")
		return
	}

	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.CreateSnippet(ctx, &req)
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC CreateSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTO(res))
}

// Get 代理详情：请求获取某个私有代码片段。
func (h *NoteHandlerGrpc) Get(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.GetSnippet(ctx, &notepb.GetSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC GetSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTO(res))
}

// Update 代理更新：修改已有代码片段。
func (h *NoteHandlerGrpc) Update(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req notepb.UpdateSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数验证失败")
		return
	}
	req.SnippetId = id

	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.UpdateSnippet(ctx, &req)
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC UpdateSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTO(res))
}

// Delete 代理删除：删除代码片段。
func (h *NoteHandlerGrpc) Delete(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.DeleteSnippet(ctx, &notepb.DeleteSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC DeleteSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Search 代理搜索：进行笔记模糊检索。
func (h *NoteHandlerGrpc) Search(c *gin.Context) {
	// TODO: 未完成 - 提取 URL Params 生成结构化的搜索语句分发
	userID, ok := h.extractUserID(c)
	if !ok { return }

	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.SearchSnippets(ctx, &notepb.SearchSnippetsRequest{Query: c.Request.URL.RawQuery})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC SearchSnippets 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTOList(res.Snippets))
}

// GetPublic 代理公开：无需鉴权的片段只读接口。
func (h *NoteHandlerGrpc) GetPublic(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.noteClient.GetPublicSnippet(c.Request.Context(), &notepb.GetPublicSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("gRPC GetPublicSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTO(res))
}

// Favorite 代理收藏：按片段ID进行用户个人收藏。
func (h *NoteHandlerGrpc) Favorite(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.FavoriteSnippet(ctx, &notepb.FavoriteSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC FavoriteSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// Unfavorite 代理取消收藏。
func (h *NoteHandlerGrpc) Unfavorite(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.UnfavoriteSnippet(ctx, &notepb.UnfavoriteSnippetRequest{SnippetId: id})
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC UnfavoriteSnippet 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// CreateFromTemplate 代理通过指定模板快速生成笔记。
func (h *NoteHandlerGrpc) CreateFromTemplate(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }

	var req notepb.CreateSnippetFromTemplateRequest
	c.ShouldBindJSON(&req)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.CreateSnippetFromTemplate(ctx, &req)
	if err != nil {
		commonlogger.Ctx(ctx, h.log).Error("gRPC CreateSnippetFromTemplate 失败", zap.Error(err))
		response.Error(c, err)
		return
	}
	response.Success(c, res)
}

// =========================================================================
// 侧边栏工作区 (Workspace Router Handlers)
// =========================================================================

// ListRecent 获取近期的流转查询列表。
func (h *NoteHandlerGrpc) ListRecent(c *gin.Context) {
	// TODO: 未完成 - API层追加可接收的条数限制 limit 参数
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListRecentSnippets(ctx, &notepb.ListSnippetsRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTOList(res.Snippets))
}

// ListShared 获取他客共享给自己的片段列表。
func (h *NoteHandlerGrpc) ListShared(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListSharedSnippets(ctx, &notepb.ListSnippetsRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTOList(res.Snippets))
}

// ListFavorites 获取用户的专属收藏树。
func (h *NoteHandlerGrpc) ListFavorites(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListFavoriteSnippets(ctx, &notepb.ListSnippetsRequest{})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, toSnippetDTOList(res.Snippets))
}

// =========================================================================
// 辅助资源群 (Groups & Tags Router Handlers)
// =========================================================================

// GetGroups 罗列树形的分组大纲。
func (h *NoteHandlerGrpc) GetGroups(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListGroups(ctx, &notepb.ListGroupsRequest{})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// CreateGroup 创建一个新的分类。
func (h *NoteHandlerGrpc) CreateGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var req notepb.CreateGroupRequest
	c.ShouldBindJSON(&req)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.CreateGroup(ctx, &req)
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// UpdateGroup 刷新一个分类节点的信息名称。
func (h *NoteHandlerGrpc) UpdateGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var req notepb.UpdateGroupRequest
	c.ShouldBindJSON(&req)
	req.GroupId, _ = strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.UpdateGroup(ctx, &req)
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// DeleteGroup 级联移除一个存在的群组分类。
func (h *NoteHandlerGrpc) DeleteGroup(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.DeleteGroup(ctx, &notepb.DeleteGroupRequest{GroupId: id})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// GetTags 获取平铺标签库。
func (h *NoteHandlerGrpc) GetTags(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListTags(ctx, &notepb.ListTagsRequest{})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// CreateTag 发行一个统一的新标签。
func (h *NoteHandlerGrpc) CreateTag(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	var req notepb.CreateTagRequest
	c.ShouldBindJSON(&req)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.CreateTag(ctx, &req)
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// DeleteTag 强制移除一个多处绑定的标签节点。
func (h *NoteHandlerGrpc) DeleteTag(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.DeleteTag(ctx, &notepb.DeleteTagRequest{TagId: id})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// =========================================================================
// 特殊行为层 (Templates & Upload Router Handlers)
// =========================================================================

// GetTemplates 加载共享库可用的公约模板。
func (h *NoteHandlerGrpc) GetTemplates(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.ListTemplates(ctx, &notepb.ListTemplatesRequest{})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// GetTemplate 读取模板详细明细以进入表单还原。
func (h *NoteHandlerGrpc) GetTemplate(c *gin.Context) {
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	res, err := h.noteClient.GetTemplate(ctx, &notepb.GetTemplateRequest{TemplateId: c.Param("id")})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}

// Upload 极简代理将 HTTP 分块流转发为 gRPC 数据包进行落盘。
func (h *NoteHandlerGrpc) Upload(c *gin.Context) {
	// TODO: 未完成 - API Gateway 应执行流式上传剥离数据包后多次传递块数据
	userID, ok := h.extractUserID(c)
	if !ok { return }
	ctx := grpcclient.WithUserID(c.Request.Context(), userID)
	// 此处为针对 gRPC 设计的 Stub 实现
	res, err := h.noteClient.UploadFile(ctx, &notepb.UploadFileRequest{Filename: "upload.file"})
	if err != nil { response.Error(c, err); return }
	response.Success(c, res)
}
