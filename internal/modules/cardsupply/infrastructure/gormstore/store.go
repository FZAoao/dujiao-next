package gormstore

import (
	"errors"
	"time"

	supplycontract "github.com/dujiao-next/internal/modules/cardsupply/contract"
	supplydomain "github.com/dujiao-next/internal/modules/cardsupply/domain"
	"gorm.io/gorm"
)

// Store 是供号机 API 凭证端口的 GORM 实现。
type Store struct {
	db *gorm.DB
}

var _ supplycontract.Store = (*Store)(nil)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (r *Store) Create(source *supplydomain.Source) error {
	if source == nil {
		return errors.New("source is nil")
	}
	return r.db.Create(source).Error
}

func (r *Store) FindByID(id uint) (*supplydomain.Source, error) {
	if id == 0 {
		return nil, errors.New("invalid source id")
	}
	var source supplydomain.Source
	if err := r.db.Where("deleted_at IS NULL").First(&source, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

func (r *Store) FindByAPIKey(apiKey string) (*supplydomain.Source, error) {
	if apiKey == "" {
		return nil, nil
	}
	var source supplydomain.Source
	if err := r.db.Where("api_key = ? AND deleted_at IS NULL", apiKey).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

func (r *Store) FindAll() ([]supplydomain.Source, error) {
	var sources []supplydomain.Source
	if err := r.db.Where("deleted_at IS NULL").Order("id desc").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

func (r *Store) Update(source *supplydomain.Source) error {
	if source == nil || source.ID == 0 {
		return errors.New("invalid source")
	}
	return r.db.Save(source).Error
}

func (r *Store) UpdateLastUsed(id uint, at time.Time) error {
	if id == 0 {
		return errors.New("invalid source id")
	}
	return r.db.Model(&supplydomain.Source{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{"last_used_at": at, "updated_at": at}).Error
}

func (r *Store) UpdateCallResult(id uint, at time.Time, success bool, errorCode string) error {
	if id == 0 {
		return errors.New("invalid source id")
	}
	updates := map[string]interface{}{"last_used_at": at, "updated_at": at}
	if success {
		updates["last_success_at"] = at
		updates["last_error_code"] = ""
	} else {
		updates["last_failure_at"] = at
		updates["last_error_code"] = errorCode
	}
	return r.db.Model(&supplydomain.Source{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates).Error
}

func (r *Store) Delete(id uint, at time.Time) error {
	if id == 0 {
		return errors.New("invalid source id")
	}
	if at.IsZero() {
		at = time.Now()
	}
	return r.db.Model(&supplydomain.Source{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{"deleted_at": at, "updated_at": at}).Error
}
