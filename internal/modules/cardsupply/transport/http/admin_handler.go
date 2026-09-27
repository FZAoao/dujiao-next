package cardsupplyhttp

import (
	"errors"
	"strings"

	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	cardsupplyapp "github.com/dujiao-next/internal/modules/cardsupply/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台供号机来源管理端口。
type AdminService interface {
	ListSourceDetails() ([]cardsupplyapp.SourceDetail, error)
	CreateSource(cardsupplyapp.CreateSourceInput) (*cardsupplyapp.SourceDetail, error)
	GetSourceDetail(uint) (*cardsupplyapp.SourceDetail, error)
	UpdateSource(uint, cardsupplyapp.UpdateSourceInput) (*cardsupplyapp.SourceDetail, error)
	UpdateSourceStatus(uint, int) error
	ResetSourceSecret(uint) (*cardsupplyapp.SourceDetail, error)
	DeleteSource(uint) error
}

// AuditService 是后台权限审计端口。
type AuditService interface {
	Record(auditlogapp.AuthzRecord) error
}

type createSourceRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	ProductID    uint   `json:"product_id" binding:"required"`
	SKUID        uint   `json:"sku_id" binding:"required"`
	MaxBatchSize int    `json:"max_batch_size"`
	IPAllowlist  string `json:"ip_allowlist"`
}

type updateSourceRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	ProductID    uint   `json:"product_id" binding:"required"`
	SKUID        uint   `json:"sku_id" binding:"required"`
	MaxBatchSize int    `json:"max_batch_size"`
	IPAllowlist  string `json:"ip_allowlist"`
}

type updateSourceStatusRequest struct {
	Status int `json:"status" binding:"oneof=0 1"`
}

// AdminHandler 处理后台供号机来源管理请求。
type AdminHandler struct {
	service AdminService
	audit   AuditService
}

func NewAdminHandler(service AdminService, audit AuditService) *AdminHandler {
	if service == nil {
		panic("card supply admin handler: service is nil")
	}
	return &AdminHandler{service: service, audit: audit}
}

func (h *AdminHandler) ListSources(c *gin.Context) {
	items, err := h.service.ListSourceDetails()
	if err != nil {
		respondAdminError(c, response.CodeInternal, "error.card_supply_sources_fetch_failed", err)
		return
	}
	response.Success(c, items)
}

func (h *AdminHandler) CreateSource(c *gin.Context) {
	var req createSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	adminID := c.GetUint("admin_id")
	result, err := h.service.CreateSource(cardsupplyapp.CreateSourceInput{
		Name:         req.Name,
		Description:  req.Description,
		ProductID:    req.ProductID,
		SKUID:        req.SKUID,
		MaxBatchSize: req.MaxBatchSize,
		IPAllowlist:  req.IPAllowlist,
		CreatedBy:    adminID,
	})
	if err != nil {
		respondSourceError(c, err, "error.card_supply_source_create_failed")
		return
	}
	h.recordAudit(c, "card_supply_source_create", "/admin/card-supply-sources", "POST", gin.H{
		"source_id":      result.ID,
		"name":           result.Name,
		"product_id":     result.ProductID,
		"sku_id":         result.SKUID,
		"status":         result.Status,
		"max_batch_size": result.MaxBatchSize,
		"ip_allowlist":   result.IPAllowlist,
	})
	response.Success(c, result)
}

func (h *AdminHandler) GetSource(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	result, err := h.service.GetSourceDetail(id)
	if err != nil {
		respondSourceError(c, err, "error.card_supply_source_fetch_failed")
		return
	}
	response.Success(c, result)
}

func (h *AdminHandler) UpdateSource(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.service.UpdateSource(id, cardsupplyapp.UpdateSourceInput{
		Name:         req.Name,
		Description:  req.Description,
		ProductID:    req.ProductID,
		SKUID:        req.SKUID,
		MaxBatchSize: req.MaxBatchSize,
		IPAllowlist:  req.IPAllowlist,
	})
	if err != nil {
		respondSourceError(c, err, "error.card_supply_source_update_failed")
		return
	}
	h.recordAudit(c, "card_supply_source_update", "/admin/card-supply-sources/:id", "PUT", gin.H{
		"source_id":      result.ID,
		"name":           result.Name,
		"product_id":     result.ProductID,
		"sku_id":         result.SKUID,
		"status":         result.Status,
		"max_batch_size": result.MaxBatchSize,
		"ip_allowlist":   result.IPAllowlist,
	})
	response.Success(c, result)
}

func (h *AdminHandler) UpdateSourceStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updateSourceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if err := h.service.UpdateSourceStatus(id, req.Status); err != nil {
		respondSourceError(c, err, "error.card_supply_source_update_failed")
		return
	}
	h.recordAudit(c, "card_supply_source_status_update", "/admin/card-supply-sources/:id/status", "PUT", gin.H{
		"source_id": id,
		"status":    req.Status,
	})
	response.Success(c, nil)
}

func (h *AdminHandler) ResetSourceSecret(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	result, err := h.service.ResetSourceSecret(id)
	if err != nil {
		respondSourceError(c, err, "error.card_supply_source_reset_secret_failed")
		return
	}
	h.recordAudit(c, "card_supply_source_reset_secret", "/admin/card-supply-sources/:id/reset-secret", "POST", gin.H{
		"source_id": id,
	})
	response.Success(c, result)
}

func (h *AdminHandler) DeleteSource(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.service.DeleteSource(id); err != nil {
		respondSourceError(c, err, "error.card_supply_source_delete_failed")
		return
	}
	h.recordAudit(c, "card_supply_source_delete", "/admin/card-supply-sources/:id", "DELETE", gin.H{
		"source_id": id,
	})
	response.Success(c, nil)
}

func (h *AdminHandler) recordAudit(c *gin.Context, action, object, method string, detail gin.H) {
	if h == nil || h.audit == nil {
		return
	}
	_ = h.audit.Record(auditlogapp.AuthzRecord{
		OperatorAdminID:  c.GetUint("admin_id"),
		OperatorUsername: c.GetString("username"),
		Action:           action,
		Object:           object,
		Method:           method,
		RequestID:        strings.TrimSpace(c.GetString("request_id")),
		Detail:           jsonmap.JSON(detail),
	})
}

func respondAdminError(c *gin.Context, code int, key string, err error) {
	ginutil.RespondError(c, code, key, err)
}

func respondSourceError(c *gin.Context, err error, fallbackKey string) {
	switch {
	case errors.Is(err, cardsupplyapp.ErrNotFound), errors.Is(err, cardsupplyapp.ErrProductNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.not_found", nil)
	case errors.Is(err, cardsupplyapp.ErrInvalid), errors.Is(err, cardsupplyapp.ErrProductSKUInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	default:
		respondAdminError(c, response.CodeInternal, fallbackKey, err)
	}
}
