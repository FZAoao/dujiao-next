package nowpayments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"
	"github.com/shopspring/decimal"
)

var (
	ErrConfigInvalid    = errors.New("nowpayments config invalid")
	ErrRequestFailed    = errors.New("nowpayments request failed")
	ErrResponseInvalid  = errors.New("nowpayments response invalid")
	ErrSignatureInvalid = errors.New("nowpayments signature invalid")
)

const defaultAPIBaseURL = "https://api.nowpayments.io"

type Config struct {
	common.ExchangeRateConfig
	APIKey         string `json:"api_key"`
	IPNSecret      string `json:"ipn_secret"`
	APIBaseURL     string `json:"api_base_url"`
	IPNCallbackURL string `json:"ipn_callback_url"`
	SuccessURL     string `json:"success_url"`
	CancelURL      string `json:"cancel_url"`
	PriceCurrency  string `json:"price_currency"`
	PayCurrency    string `json:"pay_currency"`
	FixedRate      bool   `json:"is_fixed_rate"`
	FeePaidByUser  bool   `json:"is_fee_paid_by_user"`
}

type CreateInput struct {
	OrderNo     string
	Amount      string
	Currency    string
	Description string
	NotifyURL   string
	ReturnURL   string
	CancelURL   string
}

type CreateResult struct {
	PaymentID   string
	PayAddress  string
	PayAmount   string
	PayCurrency string
	Amount      string
	Currency    string
	Raw         map[string]interface{}
}

type WebhookResult struct {
	PaymentID string
	OrderNo   string
	Status    string
	Amount    string
	Currency  string
	PaidAt    *time.Time
	Raw       map[string]interface{}
}

func ParseConfig(raw map[string]interface{}) (*Config, error) {
	return common.ParseConfig[Config](raw, ErrConfigInvalid)
}

func (c *Config) Normalize() {
	if c == nil {
		return
	}
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.IPNSecret = strings.TrimSpace(c.IPNSecret)
	c.APIBaseURL = strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if c.APIBaseURL == "" {
		c.APIBaseURL = defaultAPIBaseURL
	}
	c.IPNCallbackURL = strings.TrimSpace(c.IPNCallbackURL)
	c.SuccessURL = strings.TrimSpace(c.SuccessURL)
	c.CancelURL = strings.TrimSpace(c.CancelURL)
	c.PriceCurrency = strings.ToLower(strings.TrimSpace(c.PriceCurrency))
	if c.PriceCurrency == "" {
		c.PriceCurrency = strings.ToLower(constants.SiteCurrencyDefault)
	}
	c.PayCurrency = strings.ToLower(strings.TrimSpace(c.PayCurrency))
	c.ExchangeRateConfig.NormalizeExchangeRate()
}

func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: config is nil", ErrConfigInvalid)
	}
	cfg.Normalize()
	for name, value := range map[string]string{
		"api_key": cfg.APIKey, "ipn_secret": cfg.IPNSecret,
		"ipn_callback_url": cfg.IPNCallbackURL, "success_url": cfg.SuccessURL,
		"cancel_url": cfg.CancelURL,
	} {
		if value == "" {
			return fmt.Errorf("%w: %s is required", ErrConfigInvalid, name)
		}
	}
	for name, value := range map[string]string{"api_base_url": cfg.APIBaseURL, "ipn_callback_url": cfg.IPNCallbackURL, "success_url": cfg.SuccessURL, "cancel_url": cfg.CancelURL} {
		if _, err := url.ParseRequestURI(value); err != nil {
			return fmt.Errorf("%w: %s is invalid", ErrConfigInvalid, name)
		}
	}
	return nil
}

func CreateInvoice(ctx context.Context, cfg *Config, input CreateInput) (*CreateResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.OrderNo) == "" || strings.TrimSpace(input.Amount) == "" {
		return nil, fmt.Errorf("%w: order input is invalid", ErrConfigInvalid)
	}
	priceAmount := strings.TrimSpace(input.Amount)
	priceCurrency := strings.ToLower(strings.TrimSpace(input.Currency))
	if priceCurrency == "" {
		priceCurrency = cfg.PriceCurrency
	}
	if cfg.NeedsCurrencyConversion() {
		converted, currency, err := cfg.ConvertAmount(priceAmount, priceCurrency, 2)
		if err != nil {
			return nil, err
		}
		priceAmount, priceCurrency = converted, strings.ToLower(currency)
	}
	payload := map[string]interface{}{
		"price_amount":        priceAmount,
		"price_currency":      priceCurrency,
		"pay_currency":        cfg.PayCurrency,
		"order_id":            input.OrderNo,
		"order_description":   input.Description,
		"ipn_callback_url":    cfg.IPNCallbackURL,
		"success_url":         firstNonEmpty(input.ReturnURL, cfg.SuccessURL),
		"cancel_url":          firstNonEmpty(input.CancelURL, cfg.CancelURL),
		"is_fixed_rate":       cfg.FixedRate,
		"is_fee_paid_by_user": cfg.FeePaidByUser,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request failed", ErrRequestFailed)
	}
	respBody, status, err := doJSONRequest(ctx, cfg, http.MethodPost, "/v1/payment", body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrResponseInvalid, status)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("%w: decode response failed", ErrResponseInvalid)
	}
	result := &CreateResult{Raw: raw, Amount: priceAmount, Currency: priceCurrency}
	result.PaymentID = firstNonEmpty(readString(raw, "payment_id"), readString(raw, "id"))
	result.PayAddress = readString(raw, "pay_address")
	result.PayAmount = readString(raw, "pay_amount")
	result.PayCurrency = strings.ToLower(readString(raw, "pay_currency"))
	if result.PaymentID == "" || result.PayAddress == "" || result.PayAmount == "" {
		return nil, fmt.Errorf("%w: missing payment_id, pay_address or pay_amount", ErrResponseInvalid)
	}
	return result, nil
}

func VerifyAndParseWebhook(cfg *Config, headers map[string]string, body []byte, now time.Time) (*WebhookResult, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	signature := ""
	for key, value := range headers {
		if strings.EqualFold(key, "x-nowpayments-sig") {
			signature = strings.TrimSpace(value)
			break
		}
	}
	if signature == "" {
		return nil, ErrSignatureInvalid
	}
	var raw map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON", ErrResponseInvalid)
	}
	canonical, err := canonicalJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: canonicalize body failed", ErrResponseInvalid)
	}
	digest := hmac.New(sha512.New, []byte(cfg.IPNSecret))
	_, _ = digest.Write(canonical)
	expected := hex.EncodeToString(digest.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return nil, ErrSignatureInvalid
	}
	result := &WebhookResult{Raw: raw, PaymentID: readString(raw, "payment_id"), OrderNo: readString(raw, "order_id"), Status: readString(raw, "payment_status"), Amount: readString(raw, "price_amount"), Currency: strings.ToUpper(readString(raw, "price_currency"))}
	if result.PaymentID == "" || result.OrderNo == "" {
		return nil, fmt.Errorf("%w: missing payment_id or order_id", ErrResponseInvalid)
	}
	for _, key := range []string{"updated_at", "created_at"} {
		if value := strings.TrimSpace(readString(raw, key)); value != "" {
			if parsed, err := time.Parse(time.RFC3339, value); err == nil {
				result.PaidAt = &parsed
				break
			}
		}
	}
	return result, nil
}

func Status(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "finished":
		return constants.PaymentStatusSuccess
	case "failed", "refunded", "expired", "partially_paid":
		return constants.PaymentStatusFailed
	case "waiting", "confirming", "confirmed", "sending":
		return constants.PaymentStatusPending
	default:
		return ""
	}
}

func doJSONRequest(ctx context.Context, cfg *Config, method, path string, body []byte) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, cfg.APIBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: build request failed", ErrRequestFailed)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: http request failed", ErrRequestFailed)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%w: read response failed", ErrRequestFailed)
	}
	return responseBody, resp.StatusCode, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readString(raw map[string]interface{}, key string) string {
	value := raw[key]
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

// canonicalJSON matches NOWPayments' IPN signing rule: recursively sort object
// keys before serializing the payload for HMAC verification.
func canonicalJSON(value interface{}) ([]byte, error) {
	switch current := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var builder bytes.Buffer
		builder.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				builder.WriteByte(',')
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			encodedValue, err := canonicalJSON(current[key])
			if err != nil {
				return nil, err
			}
			builder.Write(encodedKey)
			builder.WriteByte(':')
			builder.Write(encodedValue)
		}
		builder.WriteByte('}')
		return builder.Bytes(), nil
	case []interface{}:
		var builder bytes.Buffer
		builder.WriteByte('[')
		for index, item := range current {
			if index > 0 {
				builder.WriteByte(',')
			}
			encodedItem, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			builder.Write(encodedItem)
		}
		builder.WriteByte(']')
		return builder.Bytes(), nil
	default:
		return json.Marshal(value)
	}
}

var _ = decimal.Zero
