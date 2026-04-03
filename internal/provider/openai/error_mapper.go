package openai

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"

	domain "neo-code/internal/provider"
)

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

func mapResponseFailure(resp responses.Response) error {
	message := strings.TrimSpace(resp.Error.Message)
	if message == "" {
		message = "response failed"
	}

	providerErr := &domain.ProviderError{
		Code:      mapResponseFailureCode(resp.Error.Code),
		Message:   message,
		Retryable: false,
	}

	switch providerErr.Code {
	case domain.ErrorCodeRateLimit, domain.ErrorCodeServer, domain.ErrorCodeTimeout:
		providerErr.Retryable = true
	}

	return providerErr
}

func mapResponseFailureCode(code responses.ResponseErrorCode) domain.ProviderErrorCode {
	switch code {
	case responses.ResponseErrorCodeRateLimitExceeded:
		return domain.ErrorCodeRateLimit
	case responses.ResponseErrorCodeServerError:
		return domain.ErrorCodeServer
	case responses.ResponseErrorCodeVectorStoreTimeout:
		return domain.ErrorCodeTimeout
	case responses.ResponseErrorCodeInvalidPrompt,
		responses.ResponseErrorCodeInvalidImage,
		responses.ResponseErrorCodeInvalidImageFormat,
		responses.ResponseErrorCodeInvalidBase64Image,
		responses.ResponseErrorCodeInvalidImageURL,
		responses.ResponseErrorCodeImageTooLarge,
		responses.ResponseErrorCodeImageTooSmall,
		responses.ResponseErrorCodeImageParseError,
		responses.ResponseErrorCodeImageContentPolicyViolation,
		responses.ResponseErrorCodeInvalidImageMode,
		responses.ResponseErrorCodeImageFileTooLarge,
		responses.ResponseErrorCodeUnsupportedImageMediaType,
		responses.ResponseErrorCodeEmptyImageFile,
		responses.ResponseErrorCodeFailedToDownloadImage,
		responses.ResponseErrorCodeImageFileNotFound:
		return domain.ErrorCodeClient
	default:
		return domain.ErrorCodeUnknown
	}
}
