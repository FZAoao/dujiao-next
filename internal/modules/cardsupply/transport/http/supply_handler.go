package cardsupplyhttp

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	cardsupplyapp "github.com/dujiao-next/internal/modules/cardsupply/application"
	supplydomain "github.com/dujiao-next/internal/modules/cardsupply/domain"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

const (
	SupplySourceContextKey = "card_supply_source"
	SupplyRequestHashKey   = "card_supply_request_hash"
	SupplyRawBodyKey       = "card_supply_raw_body"
	SupplyMaxBodySize      = 2 << 20
)

type ingestRequest struct {
	RequestID string   `json:"request_id"`
	Secrets   []string `json:"secrets"`
	Note      string   `json:"note"`
}

// SupplyHandler 处理供号机入库 API。
type SupplyHandler struct {
	service *cardsupplyapp.Service
}

func NewSupplyHandler(service *cardsupplyapp.Service) *SupplyHandler {
	if service == nil {
		panic("card supply handler: service is nil")
	}
	return &SupplyHandler{service: service}
}

// Ingest 将已通过签名鉴权的卡密批量写入 source 绑定的 SKU。
func (h *SupplyHandler) Ingest(c *gin.Context) {
	source, ok := c.Get(SupplySourceContextKey)
	if !ok {
		respondSupplyError(c, http.StatusUnauthorized, response.CodeUnauthorized, "supply_unauthorized", "unauthorized")
		return
	}
	sourceEntity, ok := source.(*supplydomain.Source)
	if !ok || sourceEntity == nil {
		respondSupplyError(c, http.StatusUnauthorized, response.CodeUnauthorized, "supply_unauthorized", "unauthorized")
		return
	}

	success := false
	resultErrorCode := "internal_error"
	defer func() {
		go func() {
			if err := h.service.MarkCallResult(sourceEntity.ID, success, resultErrorCode); err != nil {
				// Do not expose or log request body/card secrets here.
				_ = err
			}
		}()
	}()

	var req ingestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resultErrorCode = "validation_error"
		respondSupplyError(c, http.StatusBadRequest, response.CodeBadRequest, resultErrorCode, "invalid request body")
		return
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" || len(requestID) > 128 || len(req.Secrets) == 0 {
		resultErrorCode = "validation_error"
		respondSupplyError(c, http.StatusBadRequest, response.CodeBadRequest, resultErrorCode, "request_id and secrets are required")
		return
	}
	for _, secret := range req.Secrets {
		trimmed := strings.TrimSpace(secret)
		if trimmed == "" || strings.ContainsAny(trimmed, "\r\n") {
			resultErrorCode = "validation_error"
			respondSupplyError(c, http.StatusBadRequest, response.CodeBadRequest, resultErrorCode, "each secret must be one non-empty line")
			return
		}
	}

	rawBody := rawBodyFromContext(c)
	requestHash := requestHashFromContext(c, rawBody)
	result, err := h.service.Ingest(sourceEntity, requestID, requestHash, req.Note, req.Secrets)
	if err != nil {
		resultErrorCode = supplyErrorCode(err)
		respondSupplyAppError(c, err)
		return
	}
	resultErrorCode = ""
	success = true
	response.ChannelSuccess(c, result)
}

func rawBodyFromContext(c *gin.Context) []byte {
	if value, ok := c.Get(SupplyRawBodyKey); ok {
		if body, ok := value.([]byte); ok {
			return body
		}
	}
	return nil
}

func requestHashFromContext(c *gin.Context, body []byte) string {
	if value, ok := c.Get(SupplyRequestHashKey); ok {
		if hash, ok := value.(string); ok && strings.TrimSpace(hash) != "" {
			return hash
		}
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func supplyErrorCode(err error) string {
	switch {
	case errors.Is(err, cardsupplyapp.ErrRequestIDRequired):
		return "request_id_required"
	case errors.Is(err, cardsupplyapp.ErrRequestIDConflict):
		return "request_id_conflict"
	case errors.Is(err, cardsupplyapp.ErrBatchTooLarge):
		return "batch_too_large"
	case errors.Is(err, cardsupplyapp.ErrInvalid):
		return "validation_error"
	default:
		return "internal_error"
	}
}

func respondSupplyAppError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, cardsupplyapp.ErrRequestIDConflict):
		respondSupplyError(c, http.StatusConflict, response.CodeBadRequest, "request_id_conflict", "request_id has already been used with a different body")
	case errors.Is(err, cardsupplyapp.ErrBatchTooLarge):
		respondSupplyError(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "batch_too_large", "batch exceeds the configured maximum")
	case errors.Is(err, cardsupplyapp.ErrInvalid), errors.Is(err, cardsupplyapp.ErrRequestIDRequired):
		respondSupplyError(c, http.StatusBadRequest, response.CodeBadRequest, supplyErrorCode(err), "invalid request")
	default:
		respondSupplyError(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "internal error")
	}
}

func respondSupplyError(c *gin.Context, httpStatus, statusCode int, errorCode, message string) {
	response.ChannelError(c, httpStatus, statusCode, message, errorCode)
}

// Keep json imported in this file's public contract documentation examples
// stable if the request shape changes; this compile-time assertion also makes
// accidental non-JSON request types obvious during refactors.
