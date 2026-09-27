package application

import (
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	supplydomain "github.com/dujiao-next/internal/modules/cardsupply/domain"
)

type CreateSourceInput struct {
	Name         string
	Description  string
	ProductID    uint
	SKUID        uint
	MaxBatchSize int
	IPAllowlist  string
	CreatedBy    uint
}

type UpdateSourceInput struct {
	Name         string
	Description  string
	ProductID    uint
	SKUID        uint
	MaxBatchSize int
	IPAllowlist  string
}

type SourceDetail struct {
	ID            uint       `json:"id"`
	Name          string     `json:"name"`
	APIKey        string     `json:"api_key"`
	APISecret     string     `json:"api_secret,omitempty"`
	ProductID     uint       `json:"product_id"`
	SKUID         uint       `json:"sku_id"`
	Status        int        `json:"status"`
	Description   string     `json:"description"`
	MaxBatchSize  int        `json:"max_batch_size"`
	IPAllowlist   string     `json:"ip_allowlist"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	LastErrorCode string     `json:"last_error_code,omitempty"`
	CreatedBy     *uint      `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type IngestResult struct {
	RequestID        string                  `json:"request_id"`
	BatchID          uint                    `json:"batch_id"`
	BatchNo          string                  `json:"batch_no"`
	ProductID        uint                    `json:"product_id"`
	SKUID            uint                    `json:"sku_id"`
	Received         int                     `json:"received"`
	Created          int                     `json:"created"`
	SkippedDuplicate int                     `json:"skipped_duplicate"`
	Replayed         bool                    `json:"replayed"`
	Batch            *cardsecretdomain.Batch `json:"-"`
}

func sourceDetail(source *supplydomain.Source, plainSecret string) *SourceDetail {
	if source == nil {
		return nil
	}
	return &SourceDetail{
		ID: source.ID, Name: source.Name, APIKey: source.APIKey, APISecret: plainSecret,
		ProductID: source.ProductID, SKUID: source.SKUID, Status: source.Status,
		Description: source.Description, MaxBatchSize: source.MaxBatchSize,
		IPAllowlist: source.IPAllowlist, LastUsedAt: source.LastUsedAt,
		LastSuccessAt: source.LastSuccessAt, LastFailureAt: source.LastFailureAt,
		LastErrorCode: source.LastErrorCode, CreatedBy: source.CreatedBy,
		CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt,
	}
}
