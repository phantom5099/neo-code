package openai

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	sdk "github.com/openai/openai-go/v3"

	domain "neo-code/internal/provider"
)

// mapProviderError translates SDK / transport errors into domain ProviderError values.
func mapProviderError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.NewTimeoutProviderError(err.Error())
	}

	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		message := strings.TrimSpace(apiErr.Message)
		if message == "" {
			message = strings.TrimSpace(err.Error())
		}
		return domain.NewProviderErrorFromStatus(apiErr.StatusCode, message)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return domain.NewTimeoutProviderError(err.Error())
		}
		return domain.NewNetworkProviderError(err.Error())
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return domain.NewTimeoutProviderError(urlErr.Error())
		}
		return domain.NewNetworkProviderError(urlErr.Error())
	}

	return err
}
