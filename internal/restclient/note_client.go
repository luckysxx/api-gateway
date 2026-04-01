package restclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	commonlogger "github.com/luckysxx/common/logger"
	"go.uber.org/zap"
)

// Snippet 表示从下游 go-note 服务获取的动态结构的笔记/代码片段对象。
// 作为一个 BFF (Backend for Frontend) 聚合层，使用 map[string]any 避免了过度解析强类型字段，
// 同时也比纯粹的 interface{} 拥有更好的代码结构清晰度。
type Snippet map[string]any

// NoteClient 定义了与 go-note 微服务进行交互的客户端接口规范。
type NoteClient interface {
	// --- 已有 ---
	GetRecentSnippets(ctx context.Context, userID int64) ([]Snippet, error)
	ListMySnippets(ctx context.Context, userID int64) ([]Snippet, error)
	CreateSnippet(ctx context.Context, userID int64, body map[string]any) (Snippet, error)
	GetSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error)
	UpdateSnippet(ctx context.Context, userID int64, snippetID string, body map[string]any) (Snippet, error)

	// --- 新增：片段扩展 ---
	DeleteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error)
	SearchSnippets(ctx context.Context, userID int64, rawQuery string) ([]Snippet, error)
	GetPublicSnippet(ctx context.Context, snippetID string) (Snippet, error)
	FavoriteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error)
	UnfavoriteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error)
	CreateSnippetFromTemplate(ctx context.Context, userID int64, body map[string]any) (Snippet, error)

	// --- 新增：工作区列表 ---
	GetSharedSnippets(ctx context.Context, userID int64) ([]Snippet, error)
	GetFavoriteSnippets(ctx context.Context, userID int64) ([]Snippet, error)

	// --- 新增：分组与标签 ---
	GetGroups(ctx context.Context, userID int64) ([]Snippet, error)
	CreateGroup(ctx context.Context, userID int64, body map[string]any) (Snippet, error)
	UpdateGroup(ctx context.Context, userID int64, groupID string, body map[string]any) (Snippet, error)
	DeleteGroup(ctx context.Context, userID int64, groupID string) (Snippet, error)

	GetTags(ctx context.Context, userID int64) ([]Snippet, error)
	CreateTag(ctx context.Context, userID int64, body map[string]any) (Snippet, error)
	DeleteTag(ctx context.Context, userID int64, tagID string) (Snippet, error)

	// --- 新增：模板与上传 ---
	GetTemplates(ctx context.Context, userID int64) ([]Snippet, error)
	GetTemplate(ctx context.Context, userID int64, templateID string) (Snippet, error)
	UploadFile(ctx context.Context, userID int64, contentType string, body io.Reader) (Snippet, error)
}

// noteClient 是 NoteClient 接口的具体实现体。
type noteClient struct {
	baseURL    string       // go-note 服务的内部基础地址，例如: "http://go-note:8080"
	httpClient *http.Client // 复用原生的 http 客户端以维持持久化连接池
	log        *zap.Logger  // 集成项目中统一的结构化日志组件
}

// NewNoteClient 构造函数：创建一个新的笔记服务客户端实例，并配置高可用、高性能的拨号器机制。
func NewNoteClient(baseURL string, log *zap.Logger) NoteClient {
	return &noteClient{
		// 移除配置中可能存在的尾部斜杠，防止由于 url 拼接导致的双斜线问题 (如 //api/v1)
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			// 全局请求超时防线：避免当 go-note 服务内部发生死锁或网络停滞时卡死网关的 Goroutine 资源
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				// 连接池管理：维持与下游节点的最大空闲 TCP 连接数，以追求极致的请求性能（避免频繁的三次握手）
				MaxIdleConns:        100,              // 全局最大空闲连接
				MaxIdleConnsPerHost: 100,              // 单个目标主机的最大空闲连接
				IdleConnTimeout:     90 * time.Second, // 空闲连接的存活超时时间
			},
		},
		log: log,
	}
}

// ──────────────────────────────────────────────────
// 内部工具方法
// ──────────────────────────────────────────────────

// doRequest 封装通用的 HTTP 请求执行逻辑，减少重复代码。
func (c *noteClient) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		commonlogger.Ctx(req.Context(), c.log).Error("调用 go-note 微服务发生网络层通信异常",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("go-note 服务请求发送失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 go-note 响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		commonlogger.Ctx(req.Context(), c.log).Warn("go-note 服务返回了异常的非 200 状态码",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(bodyBytes)),
		)
		return nil, fmt.Errorf("go-note 服务返回错误状态码 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}

// decodeSingle 解析下游返回的单条记录响应 { "code": ..., "msg": ..., "data": {...} }
func (c *noteClient) decodeSingle(ctx context.Context, body []byte) (Snippet, error) {
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data Snippet `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 go-note 返回体发生解码错误: %w", err)
	}
	if result.Code != 0 && result.Code != 200 {
		commonlogger.Ctx(ctx, c.log).Warn("go-note 服务上抛了业务模块异常规则",
			zap.Int("code", result.Code),
			zap.String("msg", result.Msg),
		)
		return nil, fmt.Errorf("go-note 内部业务模块阻断异常: [%d] %s", result.Code, result.Msg)
	}
	return result.Data, nil
}

// decodeList 解析下游返回的列表响应 { "code": ..., "msg": ..., "data": [...] }
func (c *noteClient) decodeList(ctx context.Context, body []byte) ([]Snippet, error) {
	var result struct {
		Code int     `json:"code"`
		Msg  string  `json:"msg"`
		Data []Snippet `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 go-note 返回体发生解码错误: %w", err)
	}
	if result.Code != 0 && result.Code != 200 {
		commonlogger.Ctx(ctx, c.log).Warn("go-note 服务上抛了业务模块异常规则",
			zap.Int("code", result.Code),
			zap.String("msg", result.Msg),
		)
		return nil, fmt.Errorf("go-note 内部业务模块阻断异常: [%d] %s", result.Code, result.Msg)
	}
	return result.Data, nil
}

// newJSONRequest 构建一个携带 JSON body 的 HTTP 请求。
func (c *noteClient) newJSONRequest(ctx context.Context, method, url string, userID int64, body map[string]any) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("构建笔记服务请求失败: %w", err)
	}

	req.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

// ──────────────────────────────────────────────────
// 接口实现
// ──────────────────────────────────────────────────

// GetRecentSnippets 实现接口方法，执行对 /api/v1/snippets 的远程 GET 调用（Dashboard 聚合专用）。
func (c *noteClient) GetRecentSnippets(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/me/snippets", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建笔记服务请求失败: %w", err)
	}
	req.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))
	req.Header.Set("Accept", "application/json")

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.decodeList(ctx, body)
}

// ListMySnippets 获取当前用户的笔记列表。
func (c *noteClient) ListMySnippets(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/me/snippets", c.baseURL)

	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.decodeList(ctx, body)
}

// CreateSnippet 创建一条新笔记。
func (c *noteClient) CreateSnippet(ctx context.Context, userID int64, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets", c.baseURL)

	req, err := c.newJSONRequest(ctx, http.MethodPost, reqURL, userID, payload)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.decodeSingle(ctx, body)
}

// GetSnippet 获取单条笔记详情。
func (c *noteClient) GetSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/%s", c.baseURL, snippetID)

	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.decodeSingle(ctx, body)
}

// UpdateSnippet 更新一条已有笔记。
func (c *noteClient) UpdateSnippet(ctx context.Context, userID int64, snippetID string, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/%s", c.baseURL, snippetID)

	req, err := c.newJSONRequest(ctx, http.MethodPut, reqURL, userID, payload)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	return c.decodeSingle(ctx, body)
}

// --- 新增：片段扩展 ---

func (c *noteClient) DeleteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/%s", c.baseURL, snippetID)
	req, err := c.newJSONRequest(ctx, http.MethodDelete, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) SearchSnippets(ctx context.Context, userID int64, rawQuery string) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/search?%s", c.baseURL, rawQuery)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

func (c *noteClient) GetPublicSnippet(ctx context.Context, snippetID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/public/snippets/%s", c.baseURL, snippetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil { return nil, err }
	req.Header.Set("Accept", "application/json")
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) FavoriteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/%s/favorite", c.baseURL, snippetID)
	req, err := c.newJSONRequest(ctx, http.MethodPost, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) UnfavoriteSnippet(ctx context.Context, userID int64, snippetID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/%s/favorite", c.baseURL, snippetID)
	req, err := c.newJSONRequest(ctx, http.MethodDelete, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) CreateSnippetFromTemplate(ctx context.Context, userID int64, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/snippets/from-template", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodPost, reqURL, userID, payload)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

// --- 新增：工作区列表 ---

func (c *noteClient) GetSharedSnippets(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/me/snippets/shared", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

func (c *noteClient) GetFavoriteSnippets(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/me/snippets/favorites", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

// --- 新增：分组与标签 ---

func (c *noteClient) GetGroups(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/groups", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

func (c *noteClient) CreateGroup(ctx context.Context, userID int64, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/groups", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodPost, reqURL, userID, payload)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) UpdateGroup(ctx context.Context, userID int64, groupID string, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/groups/%s", c.baseURL, groupID)
	req, err := c.newJSONRequest(ctx, http.MethodPut, reqURL, userID, payload)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) DeleteGroup(ctx context.Context, userID int64, groupID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/groups/%s", c.baseURL, groupID)
	req, err := c.newJSONRequest(ctx, http.MethodDelete, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) GetTags(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/tags", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

func (c *noteClient) CreateTag(ctx context.Context, userID int64, payload map[string]any) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/tags", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodPost, reqURL, userID, payload)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) DeleteTag(ctx context.Context, userID int64, tagID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/tags/%s", c.baseURL, tagID)
	req, err := c.newJSONRequest(ctx, http.MethodDelete, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

// --- 新增：模板与上传 ---

func (c *noteClient) GetTemplates(ctx context.Context, userID int64) ([]Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/templates", c.baseURL)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeList(ctx, body)
}

func (c *noteClient) GetTemplate(ctx context.Context, userID int64, templateID string) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/templates/%s", c.baseURL, templateID)
	req, err := c.newJSONRequest(ctx, http.MethodGet, reqURL, userID, nil)
	if err != nil { return nil, err }
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}

func (c *noteClient) UploadFile(ctx context.Context, userID int64, contentType string, reqBody io.Reader) (Snippet, error) {
	reqURL := fmt.Sprintf("%s/api/v1/uploads", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, reqBody)
	if err != nil { return nil, err }
	req.Header.Set("X-User-Id", fmt.Sprintf("%d", userID))
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	body, err := c.doRequest(req)
	if err != nil { return nil, err }
	return c.decodeSingle(ctx, body)
}
