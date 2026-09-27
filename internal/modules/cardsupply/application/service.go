package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/crypto"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cardsupplycontract "github.com/dujiao-next/internal/modules/cardsupply/contract"
	supplydomain "github.com/dujiao-next/internal/modules/cardsupply/domain"
	"github.com/dujiao-next/internal/upstream"
)

const (
	DefaultMaxBatchSize        = 500
	MaxMaxBatchSize            = 5000
	maxSourceDescriptionLength = 500
	maxSourceIPAllowlistLength = 4096
)

// Service 管理供号机 API 凭证，并将已鉴权请求交给卡密库存服务。
type Service struct {
	store          cardsupplycontract.Store
	productRepo    cardsecretcontract.ProductRepository
	productSKURepo cardsecretcontract.ProductSKURepository
	cardSecrets    *cardsecretapp.Service
	encKey         []byte
}

func NewService(store cardsupplycontract.Store, productRepo cardsecretcontract.ProductRepository, productSKURepo cardsecretcontract.ProductSKURepository, cardSecrets *cardsecretapp.Service, appSecretKey string) *Service {
	return &Service{
		store:          store,
		productRepo:    productRepo,
		productSKURepo: productSKURepo,
		cardSecrets:    cardSecrets,
		encKey:         crypto.DeriveKey(appSecretKey),
	}
}

func (s *Service) CreateSource(input CreateSourceInput) (*SourceDetail, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 100 || input.ProductID == 0 || input.SKUID == 0 {
		return nil, ErrInvalid
	}
	description := strings.TrimSpace(input.Description)
	if len(description) > maxSourceDescriptionLength {
		return nil, ErrInvalid
	}
	ipAllowlist, err := normalizeIPAllowlist(input.IPAllowlist)
	if err != nil {
		return nil, err
	}
	if err := s.validateProductSKU(input.ProductID, input.SKUID); err != nil {
		return nil, err
	}
	maxBatchSize, err := normalizeMaxBatchSize(input.MaxBatchSize)
	if err != nil {
		return nil, err
	}

	apiKey, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate card supply api key: %w", err)
	}
	plainSecret, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate card supply api secret: %w", err)
	}
	encryptedSecret, err := crypto.Encrypt(s.encKey, plainSecret)
	if err != nil {
		return nil, ErrSecretEncryptFailed
	}

	now := time.Now()
	source := &supplydomain.Source{
		Name:         name,
		APIKey:       apiKey,
		APISecret:    encryptedSecret,
		ProductID:    input.ProductID,
		SKUID:        input.SKUID,
		Status:       supplydomain.StatusActive,
		Description:  description,
		MaxBatchSize: maxBatchSize,
		IPAllowlist:  ipAllowlist,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if input.CreatedBy > 0 {
		source.CreatedBy = &input.CreatedBy
	}
	if s.store == nil {
		return nil, ErrSourceCreateFailed
	}
	if err := s.store.Create(source); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSourceCreateFailed, err)
	}
	return sourceDetail(source, plainSecret), nil
}

func (s *Service) ListSourceDetails() ([]SourceDetail, error) {
	if s.store == nil {
		return nil, ErrSourceCreateFailed
	}
	sources, err := s.store.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]SourceDetail, 0, len(sources))
	for i := range sources {
		result = append(result, *sourceDetail(&sources[i], ""))
	}
	return result, nil
}

func (s *Service) GetSourceDetail(id uint) (*SourceDetail, error) {
	source, err := s.findSource(id)
	if err != nil {
		return nil, err
	}
	return sourceDetail(source, ""), nil
}

func (s *Service) UpdateSource(id uint, input UpdateSourceInput) (*SourceDetail, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 100 || input.ProductID == 0 || input.SKUID == 0 {
		return nil, ErrInvalid
	}
	description := strings.TrimSpace(input.Description)
	if len(description) > maxSourceDescriptionLength {
		return nil, ErrInvalid
	}
	ipAllowlist, err := normalizeIPAllowlist(input.IPAllowlist)
	if err != nil {
		return nil, err
	}
	if err := s.validateProductSKU(input.ProductID, input.SKUID); err != nil {
		return nil, err
	}
	maxBatchSize, err := normalizeMaxBatchSize(input.MaxBatchSize)
	if err != nil {
		return nil, err
	}
	source, err := s.findSource(id)
	if err != nil {
		return nil, err
	}
	source.Name = name
	source.Description = description
	source.ProductID = input.ProductID
	source.SKUID = input.SKUID
	source.MaxBatchSize = maxBatchSize
	source.IPAllowlist = ipAllowlist
	source.UpdatedAt = time.Now()
	if err := s.store.Update(source); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSourceUpdateFailed, err)
	}
	return sourceDetail(source, ""), nil
}

func (s *Service) UpdateSourceStatus(id uint, status int) error {
	if status != supplydomain.StatusActive && status != supplydomain.StatusDisabled {
		return ErrInvalid
	}
	source, err := s.findSource(id)
	if err != nil {
		return err
	}
	source.Status = status
	source.UpdatedAt = time.Now()
	if err := s.store.Update(source); err != nil {
		return fmt.Errorf("%w: %v", ErrSourceUpdateFailed, err)
	}
	return nil
}

func (s *Service) ResetSourceSecret(id uint) (*SourceDetail, error) {
	source, err := s.findSource(id)
	if err != nil {
		return nil, err
	}
	plainSecret, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate card supply api secret: %w", err)
	}
	encryptedSecret, err := crypto.Encrypt(s.encKey, plainSecret)
	if err != nil {
		return nil, ErrSecretEncryptFailed
	}
	source.APISecret = encryptedSecret
	source.UpdatedAt = time.Now()
	if err := s.store.Update(source); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSourceUpdateFailed, err)
	}
	return sourceDetail(source, plainSecret), nil
}

func (s *Service) DeleteSource(id uint) error {
	source, err := s.findSource(id)
	if err != nil {
		return err
	}
	if err := s.store.Delete(source.ID, time.Now()); err != nil {
		return fmt.Errorf("%w: %v", ErrSourceDeleteFailed, err)
	}
	return nil
}

// VerifyRequest authenticates a supply API request. Callers should return a
// generic unauthorized response for ErrNotFound/ErrSignatureInvalid to avoid
// revealing whether an API key exists.
func (s *Service) VerifyRequest(apiKey, signature string, timestamp int64, method, path string, body []byte, remoteIP string) (*supplydomain.Source, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(signature) == "" {
		return nil, ErrSignatureInvalid
	}
	if !upstream.IsTimestampValid(timestamp) {
		return nil, ErrTimestampExpired
	}
	source, err := s.store.FindByAPIKey(strings.TrimSpace(apiKey))
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrNotFound
	}
	if source.Status != supplydomain.StatusActive {
		return nil, ErrDisabled
	}
	if !isIPAllowed(source.IPAllowlist, remoteIP) {
		return nil, ErrIPNotAllowed
	}
	plainSecret, err := crypto.Decrypt(s.encKey, source.APISecret)
	if err != nil {
		return nil, ErrSecretDecryptFailed
	}
	if !upstream.Verify(plainSecret, method, path, strings.TrimSpace(signature), timestamp, body) {
		return nil, ErrSignatureInvalid
	}
	return source, nil
}

func (s *Service) MarkCallResult(sourceID uint, success bool, errorCode string) error {
	if sourceID == 0 || s.store == nil {
		return nil
	}
	return s.store.UpdateCallResult(sourceID, time.Now(), success, strings.TrimSpace(errorCode))
}

// Ingest validates idempotency and delegates the actual inventory transaction
// to cardsecret.Service.
func (s *Service) Ingest(source *supplydomain.Source, requestID, requestHash, note string, secrets []string) (*IngestResult, error) {
	if source == nil || source.ID == 0 || source.ProductID == 0 || source.SKUID == 0 {
		return nil, ErrInvalid
	}
	requestID = strings.TrimSpace(requestID)
	requestHash = strings.TrimSpace(requestHash)
	if requestID == "" {
		return nil, ErrRequestIDRequired
	}
	if requestHash == "" {
		return nil, ErrInvalid
	}
	if source.MaxBatchSize <= 0 {
		source.MaxBatchSize = DefaultMaxBatchSize
	}
	if countNormalizedSecrets(secrets) > source.MaxBatchSize {
		return nil, ErrBatchTooLarge
	}
	if s.cardSecrets == nil {
		return nil, ErrIngestFailed
	}

	existing, err := s.cardSecrets.FindSupplyBatch(source.ID, requestID)
	if err != nil {
		return nil, ErrIngestFailed
	}
	if existing != nil {
		if existing.RequestHash != requestHash {
			return nil, ErrRequestIDConflict
		}
		return resultFromBatch(existing, true), nil
	}

	created, err := s.cardSecrets.CreateSupplyCardSecretBatch(cardsecretapp.SupplyCardSecretBatchInput{
		ProductID:         source.ProductID,
		SKUID:             source.SKUID,
		SupplySourceID:    source.ID,
		ExternalRequestID: requestID,
		RequestHash:       requestHash,
		Secrets:           secrets,
		Note:              note,
	})
	if err != nil {
		// A concurrent request may have won the unique (source, request_id)
		// constraint. Re-read it and turn that race into a normal replay.
		if existing, lookupErr := s.cardSecrets.FindSupplyBatch(source.ID, requestID); lookupErr == nil && existing != nil {
			if existing.RequestHash != requestHash {
				return nil, ErrRequestIDConflict
			}
			return resultFromBatch(existing, true), nil
		}
		return nil, ErrIngestFailed
	}
	return &IngestResult{
		RequestID:        requestID,
		BatchID:          created.Batch.ID,
		BatchNo:          created.Batch.BatchNo,
		ProductID:        created.Batch.ProductID,
		SKUID:            created.Batch.SKUID,
		Received:         created.Received,
		Created:          created.Created,
		SkippedDuplicate: created.SkippedDuplicate,
		Replayed:         false,
		Batch:            created.Batch,
	}, nil
}

func resultFromBatch(batch *cardsecretdomain.Batch, replayed bool) *IngestResult {
	if batch == nil {
		return nil
	}
	return &IngestResult{
		RequestID:        batch.ExternalRequestID,
		BatchID:          batch.ID,
		BatchNo:          batch.BatchNo,
		ProductID:        batch.ProductID,
		SKUID:            batch.SKUID,
		Received:         batch.RequestedCount,
		Created:          batch.TotalCount,
		SkippedDuplicate: batch.DuplicateCount,
		Replayed:         replayed,
		Batch:            batch,
	}
}

func (s *Service) findSource(id uint) (*supplydomain.Source, error) {
	if id == 0 || s.store == nil {
		return nil, ErrNotFound
	}
	source, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrNotFound
	}
	return source, nil
}

func (s *Service) validateProductSKU(productID, skuID uint) error {
	if s.productRepo == nil || s.productSKURepo == nil || productID == 0 || skuID == 0 {
		return ErrProductSKUInvalid
	}
	product, err := s.productRepo.GetByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return ErrProductFetchFailed
	}
	if product == nil {
		return ErrProductNotFound
	}
	sku, err := s.productSKURepo.GetByID(skuID)
	if err != nil {
		return ErrProductFetchFailed
	}
	if sku == nil || sku.ProductID != productID || !sku.IsActive {
		return ErrProductSKUInvalid
	}
	return nil
}

func normalizeMaxBatchSize(value int) (int, error) {
	if value == 0 {
		return DefaultMaxBatchSize, nil
	}
	if value < 1 || value > MaxMaxBatchSize {
		return 0, ErrInvalid
	}
	return value, nil
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func countNormalizedSecrets(values []string) int {
	count := 0
	for _, value := range values {
		for _, line := range strings.Split(value, "\n") {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
	}
	return count
}

func normalizeIPAllowlist(value string) (string, error) {
	if len(value) > maxSourceIPAllowlistLength {
		return "", ErrInvalid
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' })
	entries := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		entry := part
		if ip := net.ParseIP(entry); ip != nil {
			entry = ip.String()
		} else if _, network, err := net.ParseCIDR(entry); err == nil {
			entry = network.String()
		} else {
			return "", ErrInvalid
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}
	normalized := strings.Join(entries, "\n")
	if len(normalized) > maxSourceIPAllowlistLength {
		return "", ErrInvalid
	}
	return normalized, nil
}

func isIPAllowed(allowlist, remoteIP string) bool {
	entries := strings.FieldsFunc(allowlist, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' })
	if len(entries) == 0 {
		return true
	}
	remote := net.ParseIP(strings.TrimSpace(remoteIP))
	if remote == nil {
		return false
	}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if ip := net.ParseIP(entry); ip != nil && ip.Equal(remote) {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(remote) {
			return true
		}
	}
	return false
}
