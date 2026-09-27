package application

import "errors"

var (
	ErrNotFound            = errors.New("card supply source not found")
	ErrInvalid             = errors.New("invalid card supply source")
	ErrProductNotFound     = errors.New("product not found")
	ErrProductSKUInvalid   = errors.New("product sku invalid")
	ErrProductFetchFailed  = errors.New("product fetch failed")
	ErrSourceCreateFailed  = errors.New("card supply source create failed")
	ErrSourceUpdateFailed  = errors.New("card supply source update failed")
	ErrSourceDeleteFailed  = errors.New("card supply source delete failed")
	ErrSecretEncryptFailed = errors.New("card supply secret encrypt failed")
	ErrSecretDecryptFailed = errors.New("card supply secret decrypt failed")
	ErrTimestampExpired    = errors.New("timestamp expired")
	ErrSignatureInvalid    = errors.New("signature invalid")
	ErrDisabled            = errors.New("card supply source disabled")
	ErrIPNotAllowed        = errors.New("ip not allowed")
	ErrRequestIDRequired   = errors.New("request id required")
	ErrRequestIDConflict   = errors.New("request id conflict")
	ErrBatchTooLarge       = errors.New("batch too large")
	ErrIngestFailed        = errors.New("card supply ingest failed")
)
