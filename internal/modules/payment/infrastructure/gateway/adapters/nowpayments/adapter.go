package nowpaymentsadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/nowpayments"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type nowpaymentsAdapter struct{}

func NewNowpaymentsAdapter() paymentcontract.GatewayProvider { return &nowpaymentsAdapter{} }

var (
	_ paymentcontract.GatewayProvider  = (*nowpaymentsAdapter)(nil)
	_ paymentcontract.GatewayWebhooker = (*nowpaymentsAdapter)(nil)
)

func (a *nowpaymentsAdapter) Type() string { return constants.PaymentProviderNowpayments + ":" }

func (a *nowpaymentsAdapter) parseConfig(raw jsonmap.JSON) (*nowpayments.Config, error) {
	cfg, err := nowpayments.ParseConfig(raw)
	if err != nil {
		return nil, mapNowpaymentsError(err)
	}
	if err := nowpayments.ValidateConfig(cfg); err != nil {
		return nil, mapNowpaymentsError(err)
	}
	return cfg, nil
}

func (a *nowpaymentsAdapter) parseConfigForChannel(raw jsonmap.JSON, channelType string) (*nowpayments.Config, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.PayCurrency) == "" {
		cfg.PayCurrency = strings.ToLower(strings.TrimSpace(channelType))
	}
	if cfg.PayCurrency == "" {
		return nil, fmt.Errorf("%w: pay_currency is required", paymentcontract.ErrGatewayConfigInvalid)
	}
	return cfg, nil
}

func (a *nowpaymentsAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	if strings.TrimSpace(channelType) == "" {
		return fmt.Errorf("%w: channel_type is required", paymentcontract.ErrGatewayUnsupportedChannel)
	}
	_, err := a.parseConfigForChannel(raw, channelType)
	return err
}

func (a *nowpaymentsAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfigForChannel(raw, input.ChannelType)
	if err != nil {
		return nil, err
	}
	result, err := nowpayments.CreateInvoice(ctx, cfg, nowpayments.CreateInput{
		OrderNo: input.OrderNo, Amount: input.Amount.Decimal.String(), Currency: input.Currency,
		Description: input.Subject, ReturnURL: input.ReturnURL, CancelURL: cfg.CancelURL,
	})
	if err != nil {
		return nil, mapNowpaymentsError(err)
	}
	payload := jsonmap.JSON{}
	if result.Raw != nil {
		payload = jsonmap.JSON(result.Raw)
	}
	payload["pay_address"] = result.PayAddress
	payload["pay_amount"] = result.PayAmount
	payload["pay_currency"] = result.PayCurrency
	payload["nowpayments_user_pays_fee"] = cfg.FeePaidByUser
	return &paymentcontract.GatewayCreateResult{
		ProviderRef: result.PaymentID, QRCodeURL: result.PayAddress, Payload: payload,
		AmountSent: result.Amount, CurrencySent: strings.ToUpper(result.Currency),
	}, nil
}

func (a *nowpaymentsAdapter) ParseWebhook(_ context.Context, raw jsonmap.JSON, headers map[string]string, body []byte, now time.Time) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	result, err := nowpayments.VerifyAndParseWebhook(cfg, headers, body, now)
	if err != nil {
		return nil, mapNowpaymentsError(err)
	}
	amount := money.Amount{}
	if parsed, parseErr := decimal.NewFromString(strings.TrimSpace(result.Amount)); parseErr == nil {
		amount = money.FromDecimal(parsed)
	}
	return &paymentcontract.GatewayCallbackResult{
		OrderNo: result.OrderNo, ProviderRef: result.PaymentID, Status: nowpayments.Status(result.Status),
		Amount: amount, Currency: strings.ToUpper(result.Currency), PaidAt: result.PaidAt, Payload: jsonmap.JSON(result.Raw),
	}, nil
}

func mapNowpaymentsError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, nowpayments.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, nowpayments.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	case errors.Is(err, nowpayments.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, nowpayments.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	default:
		return err
	}
}
