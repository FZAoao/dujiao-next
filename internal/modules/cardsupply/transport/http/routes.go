package cardsupplyhttp

import (
	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册供号机来源管理路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/card-supply-sources", handler.ListSources)
	admin.POST("/card-supply-sources", handler.CreateSource)
	admin.GET("/card-supply-sources/:id", handler.GetSource)
	admin.PUT("/card-supply-sources/:id", handler.UpdateSource)
	admin.PUT("/card-supply-sources/:id/status", handler.UpdateSourceStatus)
	admin.POST("/card-supply-sources/:id/reset-secret", handler.ResetSourceSecret)
	admin.DELETE("/card-supply-sources/:id", handler.DeleteSource)
}

// RegisterSupplyRoutes 注册供号机入库接口。
func RegisterSupplyRoutes(api gin.IRoutes, handler *SupplyHandler, auth gin.HandlerFunc, rateLimit gin.HandlerFunc) {
	middlewares := make([]gin.HandlerFunc, 0, 2)
	if rateLimit != nil {
		middlewares = append(middlewares, rateLimit)
	}
	if auth != nil {
		middlewares = append(middlewares, auth)
	}
	api.POST("/card-supply/secrets", append(middlewares, handler.Ingest)...)
}
