package domain

import (
	"time"
)

// Batch 卡密导入批次实体。
type Batch struct {
	ID                uint       `gorm:"primarykey" json:"id"`                                 // 主键
	ProductID         uint       `gorm:"index;not null" json:"product_id"`                     // 商品ID
	SKUID             uint       `gorm:"column:sku_id;index;not null;default:0" json:"sku_id"` // SKU ID
	BatchNo           string     `gorm:"uniqueIndex;not null" json:"batch_no"`                 // 批次号
	Source            string     `gorm:"not null" json:"source"`                               // 来源（manual/csv/supply_api）
	TotalCount        int        `gorm:"not null" json:"total_count"`                          // 实际新建数量
	Note              string     `gorm:"type:text" json:"note"`                                // 备注
	SupplySourceID    *uint      `gorm:"index;uniqueIndex:idx_card_secret_supply_request" json:"supply_source_id,omitempty"`
	ExternalRequestID string     `gorm:"size:128;uniqueIndex:idx_card_secret_supply_request" json:"external_request_id,omitempty"`
	RequestHash       string     `gorm:"size:64;index" json:"-"` // 供号请求体指纹
	RequestedCount    int        `gorm:"not null;default:0" json:"requested_count"`
	DuplicateCount    int        `gorm:"not null;default:0" json:"duplicate_count"`
	CreatedBy         *uint      `gorm:"index" json:"created_by,omitempty"` // 创建管理员ID
	CreatedAt         time.Time  `gorm:"index" json:"created_at"`           // 创建时间
	UpdatedAt         time.Time  `gorm:"index" json:"updated_at"`           // 更新时间
	DeletedAt         *time.Time `gorm:"index" json:"-"`                    // 软删除时间
}

// TableName 指定表名
func (Batch) TableName() string {
	return "card_secret_batches"
}
