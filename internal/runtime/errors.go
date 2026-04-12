package runtime

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"time"

	"neo-code/internal/provider"
)

// handleRunError 负责把运行错误映射为最终 runtime 事件。
func (s *Service) handleRunError(ctx context.Context, runID string, sessionID string, err error) error {
	if errors.Is(err, context.Canceled) {
		s.emit(ctx, EventRunCanceled, runID, sessionID, nil)
		return context.Canceled
	}

	var providerErr *provider.ProviderError
	if errors.As(err, &providerErr) {
		log.Printf("runtime: provider error (status=%d, code=%s, retryable=%v): %s",
			providerErr.StatusCode, providerErr.Code, providerErr.Retryable, providerErr.Message)
	}

	s.emit(ctx, EventError, runID, sessionID, err.Error())
	return err
}

// isRetryableProviderError 判断 provider 错误是否允许 runtime 级重试。
func isRetryableProviderError(err error) bool {
	var providerErr *provider.ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	return providerErr.Retryable
}

// providerRetryBackoff 计算 runtime 级 provider 重试等待时间。
func providerRetryBackoff(attempt int) time.Duration {
	wait := providerRetryBaseWait << (attempt - 1)
	jitter := float64(wait) * (0.5 + rand.Float64())
	wait = time.Duration(jitter)
	if wait > providerRetryMaxWait {
		wait = providerRetryMaxWait
	}
	return wait
}
