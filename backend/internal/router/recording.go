package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/oralhistory/oralhistory/internal/config"
	"github.com/oralhistory/oralhistory/internal/constants"
	"github.com/oralhistory/oralhistory/internal/handler"
	"github.com/oralhistory/oralhistory/internal/middleware"
)

// RegisterRecordingRoutes 注册录音路由。
func RegisterRecordingRoutes(g *gin.RouterGroup, h *handler.RecordingHandler, cfg *config.Config, logger *slog.Logger) {
	group := g.Group("/recordings", middleware.Auth(cfg.JWTSecret, logger))
	{
		group.GET("", h.List)
		group.POST("", h.Create)
		group.GET("/:id", h.Get)
		group.PUT("/:id", h.Update)
		group.PUT("/:id/summary", h.UpdateSummary)
		// 提交审核：采访员发起，管理员可代操作。
		group.POST("/:id/submit-review", middleware.RBAC(logger, constants.RoleInterviewer, constants.RoleAdmin), h.SubmitForReview)
		// 审核（通过/退回）：档案员与管理员负责。
		group.POST("/:id/approve", middleware.RBAC(logger, constants.RoleArchivist, constants.RoleAdmin), h.ApproveReview)
		group.POST("/:id/reject", middleware.RBAC(logger, constants.RoleArchivist, constants.RoleAdmin), h.RejectReview)
		group.POST("/:id/audio", h.UploadAudio)
		group.GET("/:id/audio", h.PlayAudio)
		group.DELETE("/:id", h.Delete)
	}
}
