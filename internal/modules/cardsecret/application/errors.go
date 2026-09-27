package application

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrInsufficient       = errors.New("card secret insufficient")
	ErrInvalid            = errors.New("card secret invalid")
	ErrCreateFailed       = errors.New("card secret create failed")
	ErrFetchFailed        = errors.New("card secret fetch failed")
	ErrUpdateFailed       = errors.New("card secret update failed")
	ErrDeleteFailed       = errors.New("card secret delete failed")
	ErrBatchCreateFailed  = errors.New("card secret batch create failed")
	ErrBatchFetchFailed   = errors.New("card secret batch fetch failed")
	ErrImportFailed       = errors.New("card secret import failed")
	ErrStatsFailed        = errors.New("card secret stats failed")
	ErrProductFetchFailed = errors.New("product fetch failed")
	ErrProductNotFound    = errors.New("product not found")
	ErrProductSKURequired = errors.New("product sku required")
	ErrProductSKUInvalid  = errors.New("product sku invalid")
)

// cardSecretFingerprint identifies a normalized secret within one product SKU.
// The product and SKU scope prevents the same opaque text from colliding across
// unrelated products while keeping the persisted value irreversible.
func cardSecretFingerprint(productID, skuID uint, secret string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", productID, skuID, secret)))
	return hex.EncodeToString(digest[:])
}

// SupplyCardSecretBatchInput 描述供号机 API 的固定 SKU 入库请求。
// ProductID/SKUID 来自供号源绑定关系，调用方不能从请求体覆盖。
type SupplyCardSecretBatchInput struct {
	ProductID         uint
	SKUID             uint
	SupplySourceID    uint
	ExternalRequestID string
	RequestHash       string
	Secrets           []string
	Note              string
}

// SupplyCardSecretBatchResult 是供号入库及去重结果。
type SupplyCardSecretBatchResult struct {
	Batch            *cardsecretdomain.Batch
	Received         int
	Created          int
	SkippedDuplicate int
}

// FindSupplyBatch 按供号源和外部请求 ID 查找已处理请求。
func (s *Service) FindSupplyBatch(sourceID uint, requestID string) (*cardsecretdomain.Batch, error) {
	if sourceID == 0 || strings.TrimSpace(requestID) == "" || s.batchRepo == nil {
		return nil, nil
	}
	batch, err := s.batchRepo.FindBySupplyRequest(sourceID, strings.TrimSpace(requestID))
	if err != nil {
		return nil, ErrBatchFetchFailed
	}
	return batch, nil
}

// CreateSupplyCardSecretBatch 将供号机卡密写入库存，并在同一事务内创建批次。
func (s *Service) CreateSupplyCardSecretBatch(input SupplyCardSecretBatchInput) (*SupplyCardSecretBatchResult, error) {
	if input.ProductID == 0 || input.SKUID == 0 || input.SupplySourceID == 0 || strings.TrimSpace(input.ExternalRequestID) == "" || strings.TrimSpace(input.RequestHash) == "" {
		return nil, ErrInvalid
	}
	if s.batchRepo == nil || s.transactions == nil || s.secretRepo == nil {
		return nil, ErrBatchCreateFailed
	}

	if _, err := s.resolveCardSecretSKU(input.ProductID, input.SKUID); err != nil {
		return nil, err
	}

	normalized := normalizeSecrets(input.Secrets, false)
	if len(normalized) == 0 {
		return nil, ErrInvalid
	}

	// 同一个请求内的重复卡密只保留第一条，保持供号机提交顺序。
	unique := make([]string, 0, len(normalized))
	seen := make(map[string]struct{}, len(normalized))
	for _, secret := range normalized {
		if _, exists := seen[secret]; exists {
			continue
		}
		seen[secret] = struct{}{}
		unique = append(unique, secret)
	}

	fingerprints := make([]string, 0, len(unique))
	for _, secret := range unique {
		fingerprints = append(fingerprints, cardSecretFingerprint(input.ProductID, input.SKUID, secret))
	}

	now := time.Now()
	batch := &cardsecretdomain.Batch{
		ProductID:         input.ProductID,
		SKUID:             input.SKUID,
		BatchNo:           generateSupplyBatchNo(),
		Source:            constants.CardSecretSourceSupplyAPI,
		TotalCount:        0,
		Note:              strings.TrimSpace(input.Note),
		SupplySourceID:    uintPointer(input.SupplySourceID),
		ExternalRequestID: strings.TrimSpace(input.ExternalRequestID),
		RequestHash:       strings.TrimSpace(input.RequestHash),
		RequestedCount:    len(normalized),
		DuplicateCount:    len(normalized) - len(unique),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	result := &SupplyCardSecretBatchResult{Batch: batch, Received: len(normalized)}
	err := s.transactions.Transaction(func(secretRepo cardsecretcontract.Repository, batchRepo cardsecretcontract.BatchRepository) error {
		fingerprintSet, err := secretRepo.ListExistingFingerprints(input.ProductID, input.SKUID, fingerprints)
		if err != nil {
			return fmt.Errorf("list existing card secret fingerprints: %w", err)
		}
		legacySecretSet, err := secretRepo.ListExistingSecrets(input.ProductID, input.SKUID, unique)
		if err != nil {
			return fmt.Errorf("list existing card secrets: %w", err)
		}

		items := make([]cardsecretdomain.Secret, 0, len(unique))
		for _, secret := range unique {
			fingerprint := cardSecretFingerprint(input.ProductID, input.SKUID, secret)
			if _, exists := fingerprintSet[fingerprint]; exists {
				batch.DuplicateCount++
				continue
			}
			if _, exists := legacySecretSet[secret]; exists {
				batch.DuplicateCount++
				continue
			}
			items = append(items, cardsecretdomain.Secret{
				ProductID:         input.ProductID,
				SKUID:             input.SKUID,
				BatchID:           &batch.ID,
				Secret:            secret,
				SecretFingerprint: fingerprint,
				Status:            cardsecretdomain.StatusAvailable,
				CreatedAt:         now,
				UpdatedAt:         now,
			})
		}
		batch.TotalCount = len(items)
		result.Created = len(items)
		result.SkippedDuplicate = batch.DuplicateCount

		if err := batchRepo.Create(batch); err != nil {
			return ErrBatchCreateFailed
		}
		// A duplicate-only request still gets a batch record for durable idempotency.
		if err := secretRepo.CreateBatch(items); err != nil {
			return ErrCreateFailed
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrBatchCreateFailed) {
			return nil, ErrBatchCreateFailed
		}
		if errors.Is(err, ErrCreateFailed) {
			return nil, ErrCreateFailed
		}
		return nil, ErrCreateFailed
	}
	return result, nil
}

func uintPointer(value uint) *uint { return &value }

func generateSupplyBatchNo() string {
	return fmt.Sprintf("SUPPLY-%s", time.Now().Format("20060102150405.000000000"))
}
