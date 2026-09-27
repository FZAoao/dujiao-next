package nowpayments

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestVerifyAndParseWebhook(t *testing.T) {
	body := []byte(`{"payment_id":123,"payment_status":"finished","pay_currency":"usdttrc20","price_amount":"10.00","price_currency":"cny","order_id":"ORDER-1"}`)
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha512.New, []byte("ipn-secret"))
	_, _ = mac.Write(canonical)
	signature := hex.EncodeToString(mac.Sum(nil))

	cfg := &Config{APIKey: "api-key", IPNSecret: "ipn-secret", IPNCallbackURL: "https://shop.example/ipn", SuccessURL: "https://shop.example/pay", CancelURL: "https://shop.example/pay", PayCurrency: "usdttrc20"}
	result, err := VerifyAndParseWebhook(cfg, map[string]string{"x-nowpayments-sig": signature}, body, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderNo != "ORDER-1" || result.PaymentID != "123" || Status(result.Status) != "success" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestStatus(t *testing.T) {
	for status, expected := range map[string]string{
		"finished": "success", "confirming": "pending", "failed": "failed", "expired": "failed", "unknown": "",
	} {
		if actual := Status(status); actual != expected {
			t.Fatalf("status %q: got %q, want %q", status, actual, expected)
		}
	}
}
