package epay

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestIsSupportedChannelTypeUSDT(t *testing.T) {
	for _, channelType := range []string{
		constants.PaymentChannelTypeUsdt,
		constants.PaymentChannelTypeUsdtTrc20,
	} {
		if !IsSupportedChannelType(channelType) {
			t.Fatalf("expected %q to be supported", channelType)
		}
	}
}
