package contract

import (
	"time"

	supplydomain "github.com/dujiao-next/internal/modules/cardsupply/domain"
)

// Store 持久化供号机 API 凭证。
type Store interface {
	Create(*supplydomain.Source) error
	FindByID(uint) (*supplydomain.Source, error)
	FindByAPIKey(string) (*supplydomain.Source, error)
	FindAll() ([]supplydomain.Source, error)
	Update(*supplydomain.Source) error
	UpdateLastUsed(uint, time.Time) error
	UpdateCallResult(uint, time.Time, bool, string) error
	Delete(uint, time.Time) error
}
