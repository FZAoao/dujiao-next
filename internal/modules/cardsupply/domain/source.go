package domain

import "time"

const (
	StatusDisabled = 0
	StatusActive   = 1
)

// Source 是一个固定绑定到商品 SKU 的供号机 API 凭证。
type Source struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"size:100;not null"`
	APIKey        string     `json:"api_key" gorm:"size:64;uniqueIndex;not null"`
	APISecret     string     `json:"-" gorm:"size:512;not null"` // AES-256-GCM encrypted
	ProductID     uint       `json:"product_id" gorm:"not null;index"`
	SKUID         uint       `json:"sku_id" gorm:"not null;index"`
	Status        int        `json:"status" gorm:"not null;default:1;index"`
	Description   string     `json:"description" gorm:"size:500"`
	MaxBatchSize  int        `json:"max_batch_size" gorm:"not null;default:500"`
	IPAllowlist   string     `json:"ip_allowlist" gorm:"type:text"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	LastFailureAt *time.Time `json:"last_failure_at"`
	LastErrorCode string     `json:"last_error_code" gorm:"size:100"`
	CreatedBy     *uint      `json:"created_by,omitempty" gorm:"index"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"-" gorm:"index"`
}

func (Source) TableName() string { return "card_supply_sources" }
