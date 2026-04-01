package handler

import (
	"api-gateway/internal/grpcclient"
	"api-gateway/internal/handler/dto"
	"api-gateway/internal/handler/response"
	"api-gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	authpb "github.com/luckysxx/common/proto/auth"
	commonlogger "github.com/luckysxx/common/logger"

	"go.uber.org/zap"
)

// AuthHandler 处理认证模块的 BFF 路由，通过 gRPC 调用 user-platform 的 AuthService。
type AuthHandler struct {
	authClient authpb.AuthServiceClient
	log        *zap.Logger
}

func NewAuthHandler(authClient authpb.AuthServiceClient, log *zap.Logger) *AuthHandler {
	return &AuthHandler{authClient: authClient, log: log}
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.authClient.Login(c.Request.Context(), &authpb.LoginRequest{
		Username: req.Username,
		Password: req.Password,
		AppCode:  req.AppCode,
		DeviceId: req.DeviceId,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("用户登录失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}
	response.Success(c, &dto.LoginResponse{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		UserID:       user.UserId,
		Username:     user.Username,
	})
}

// RefreshToken 刷新访问令牌（公共接口，无需 JWT 鉴权，但需要提供 refresh_token）
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	resp, err := h.authClient.RefreshToken(c.Request.Context(), &authpb.RefreshTokenRequest{
		Token: req.RefreshToken,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("刷新令牌失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

// Logout 用户退出登录（需 JWT 鉴权，从上下文获取 userID 用于日志追踪）
func (h *AuthHandler) Logout(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	_, err := h.authClient.Logout(grpcCtx, &authpb.LogoutRequest{})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("用户退出登录失败", zap.Int64("userID", userID), zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, nil)
}
