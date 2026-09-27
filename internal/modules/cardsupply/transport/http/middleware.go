package cardsupplyhttp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	cardsupplyapp "github.com/dujiao-next/internal/modules/cardsupply/application"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

const maxSupplyBodySize int64 = SupplyMaxBodySize

// CardSupplyAuthMiddleware 验证供号机 API Key、时间戳、IP 白名单和 HMAC 签名。
func CardSupplyAuthMiddleware(service *cardsupplyapp.Service) gin.HandlerFunc {
	if service == nil {
		panic("card supply auth middleware: service is nil")
	}
	return func(c *gin.Context) {
		apiKey := strings.TrimSpace(c.GetHeader(upstream.HeaderApiKey))
		timestampRaw := strings.TrimSpace(c.GetHeader(upstream.HeaderTimestamp))
		signature := strings.TrimSpace(c.GetHeader(upstream.HeaderSignature))
		if apiKey == "" || timestampRaw == "" || signature == "" {
			abortSupplyAuth(c, http.StatusUnauthorized, "supply_unauthorized", "unauthorized")
			return
		}
		timestamp, err := upstream.ParseTimestamp(timestampRaw)
		if err != nil {
			abortSupplyAuth(c, http.StatusUnauthorized, "supply_unauthorized", "unauthorized")
			return
		}

		var body []byte
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSupplyBodySize)
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				abortSupplyAuth(c, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		source, err := service.VerifyRequest(
			apiKey,
			signature,
			timestamp,
			c.Request.Method,
			c.Request.URL.Path,
			body,
			c.ClientIP(),
		)
		if err != nil {
			switch {
			case err == cardsupplyapp.ErrDisabled:
				abortSupplyAuth(c, http.StatusForbidden, "supply_source_disabled", "supply source is disabled")
			case err == cardsupplyapp.ErrIPNotAllowed:
				abortSupplyAuth(c, http.StatusForbidden, "ip_not_allowed", "client IP is not allowed")
			default:
				abortSupplyAuth(c, http.StatusUnauthorized, "supply_unauthorized", "unauthorized")
			}
			return
		}

		sum := sha256.Sum256(body)
		c.Set(SupplySourceContextKey, source)
		c.Set(SupplyRequestHashKey, hex.EncodeToString(sum[:]))
		c.Set(SupplyRawBodyKey, body)
		c.Next()
	}
}

func abortSupplyAuth(c *gin.Context, status int, errorCode, message string) {
	response.ChannelError(c, status, statusCodeForHTTP(status), message, errorCode)
	c.Abort()
}

func statusCodeForHTTP(status int) int {
	switch status {
	case http.StatusUnauthorized:
		return response.CodeUnauthorized
	case http.StatusForbidden:
		return response.CodeForbidden
	case http.StatusRequestEntityTooLarge:
		return response.CodeBadRequest
	default:
		return response.CodeInternal
	}
}

// VerifyTimestamp is exposed for focused transport tests and documentation.
func VerifyTimestamp(timestamp time.Time) bool {
	return upstream.IsTimestampValid(timestamp.Unix())
}
