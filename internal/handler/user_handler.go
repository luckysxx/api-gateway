package handler

import (
	"api-gateway/internal/grpcclient"
	"api-gateway/internal/handler/dto"
	"api-gateway/internal/handler/response"
	"api-gateway/internal/handler/validator"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/luckysxx/common/logger"
	userpb "github.com/luckysxx/common/proto/user"

	"go.uber.org/zap"
)

// UserHandler 负责处理网关侧的用户相关请求。
type UserHandler struct {
	userClient userpb.UserServiceClient
	log        *zap.Logger
}

// NewUserHandler 创建一个用户 Handler。
func NewUserHandler(userClient userpb.UserServiceClient, log *zap.Logger) *UserHandler {
	return &UserHandler{userClient: userClient, log: log}
}

// Register 用户注册
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用 validator 翻译验证错误为友好提示
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	user, err := h.userClient.Register(c.Request.Context(), &userpb.RegisterRequest{
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
	})
	if err != nil {
		commonlogger.Ctx(c.Request.Context(), h.log).Error("用户注册失败", zap.Error(err))
		// 这里可以直接抛出，因为底层 Service 已经是 Domain Error 了
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}
	response.Success(c, &dto.RegisterResponse{
		UserID:   user.UserId,
		Username: user.Username,
	})
}

// GetProfile 获取当前登录用户的个人资料
func (h *UserHandler) GetProfile(c *gin.Context) {
	// 从网关 JWT 中间件获取身份
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.GetProfile(grpcCtx, &userpb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("获取个人资料失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.GetProfileResponse{
		UserID:    resp.UserId,
		Nickname:  resp.Nickname,
		AvatarURL: resp.AvatarUrl,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	})
}

// UpdateProfile 修改个人资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "未授权")
		return
	}
	userID := val.(int64)

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := validator.TranslateValidationError(err)
		commonlogger.Ctx(c.Request.Context(), h.log).Warn("参数验证失败", zap.Error(err), zap.String("message", errMsg))
		response.BadRequest(c, errMsg)
		return
	}

	grpcCtx := grpcclient.WithUserID(c.Request.Context(), userID)
	resp, err := h.userClient.UpdateProfile(grpcCtx, &userpb.UpdateProfileRequest{
		UserId:    userID,
		Nickname:  req.Nickname,
		AvatarUrl: req.AvatarURL,
		Bio:       req.Bio,
		Birthday:  req.Birthday,
	})
	if err != nil {
		commonlogger.Ctx(grpcCtx, h.log).Error("更新个人资料失败", zap.Error(err))
		response.Error(c, validator.ConvertToHTTPError(err))
		return
	}

	response.Success(c, &dto.UpdateProfileResponse{
		UserID:    resp.UserId,
		Nickname:  resp.Nickname,
		AvatarURL: resp.AvatarUrl,
		Bio:       resp.Bio,
		Birthday:  resp.Birthday,
		UpdatedAt: resp.UpdatedAt,
	})
}
