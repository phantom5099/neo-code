package memo

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	providertypes "neo-code/internal/provider/types"
)

const autoExtractDebounce = 2 * time.Second

// AutoExtractor 负责按会话在后台调度自动提取，并处理防抖、互斥和尾随执行。
type AutoExtractor struct {
	extractor Extractor
	svc       *Service
	debounce  time.Duration
	logf      func(format string, args ...any)

	mu      sync.Mutex
	workers map[string]*autoExtractWorker
}

type autoExtractWorker struct {
	updates chan autoExtractRequest
}

type autoExtractRequest struct {
	messages []providertypes.Message
	dueAt    time.Time
	skip     bool
}

// NewAutoExtractor 创建后台自动提取调度器。
func NewAutoExtractor(extractor Extractor, svc *Service) *AutoExtractor {
	return &AutoExtractor{
		extractor: extractor,
		svc:       svc,
		debounce:  autoExtractDebounce,
		logf:      log.Printf,
		workers:   make(map[string]*autoExtractWorker),
	}
}

// Schedule 按会话维度安排一次后台自动提取，skip=true 时清空当前待执行请求。
func (a *AutoExtractor) Schedule(sessionID string, messages []providertypes.Message, skip bool) {
	if a == nil || a.extractor == nil || a.svc == nil {
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	req := autoExtractRequest{
		messages: cloneProviderMessages(messages),
		dueAt:    time.Now().Add(a.debounce),
		skip:     skip,
	}

	worker := a.ensureWorker(sessionID)
	worker.updates <- req
}

// ensureWorker 获取或创建指定会话的后台 worker。
func (a *AutoExtractor) ensureWorker(sessionID string) *autoExtractWorker {
	a.mu.Lock()
	defer a.mu.Unlock()

	if worker, ok := a.workers[sessionID]; ok {
		return worker
	}

	worker := &autoExtractWorker{
		updates: make(chan autoExtractRequest, 32),
	}
	a.workers[sessionID] = worker
	go a.runWorker(worker)
	return worker
}

// runWorker 串行处理单个会话的调度状态，避免并发提取并支持 trailing extraction。
func (a *AutoExtractor) runWorker(worker *autoExtractWorker) {
	var (
		pending *autoExtractRequest
		timer   *time.Timer
		timerCh <-chan time.Time
		running bool
		doneCh  = make(chan struct{}, 1)
	)

	for {
		select {
		case req := <-worker.updates:
			if req.skip {
				pending = nil
				stopTimer(timer)
				timer = nil
				timerCh = nil
				continue
			}

			reqCopy := req
			pending = &reqCopy
			if running {
				continue
			}

			timer = resetTimer(timer, time.Until(req.dueAt))
			timerCh = timer.C

		case <-timerCh:
			timer = nil
			timerCh = nil

			for {
				select {
				case req := <-worker.updates:
					if req.skip {
						pending = nil
						continue
					}
					reqCopy := req
					pending = &reqCopy
				default:
					goto launch
				}
			}

		launch:
			if running || pending == nil {
				continue
			}
			if wait := time.Until(pending.dueAt); wait > 0 {
				timer = resetTimer(timer, wait)
				timerCh = timer.C
				continue
			}

			current := *pending
			pending = nil
			running = true

			go func(req autoExtractRequest) {
				a.extractAndStore(req.messages)
				doneCh <- struct{}{}
			}(current)

		case <-doneCh:
			running = false
			if pending == nil {
				continue
			}

			timer = resetTimer(timer, time.Until(pending.dueAt))
			timerCh = timer.C
		}
	}
}

// extractAndStore 执行提取，并在写入前做本地批次去重和持久化级别的原子去重。
func (a *AutoExtractor) extractAndStore(messages []providertypes.Message) {
	ctx := context.Background()
	entries, err := a.extractor.Extract(ctx, messages)
	if err != nil {
		a.logError("memo: auto extract failed: %v", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entry.Source = SourceAutoExtract
		key := autoExtractDedupKey(entry)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}

		added, err := a.svc.addAutoExtractIfAbsent(ctx, entry)
		if err != nil {
			a.logError("memo: auto extract add failed: %v", err)
			continue
		}

		seen[key] = struct{}{}
		if !added {
			continue
		}
	}
}

// autoExtractDedupKey 生成自动提取条目的精确去重键。
func autoExtractDedupKey(entry Entry) string {
	title := NormalizeTitle(entry.Title)
	content := strings.TrimSpace(entry.Content)
	if !IsValidType(entry.Type) || title == "" || content == "" {
		return ""
	}
	return strings.Join([]string{string(entry.Type), title, content}, "\x1f")
}

// parseTopicSourceAndContent 从 topic 文件中解析 source frontmatter 和正文内容。
func parseTopicSourceAndContent(topic string) (string, string) {
	parts := strings.Split(topic, "---")
	if len(parts) < 3 {
		return "", strings.TrimSpace(topic)
	}

	frontmatter := parts[1]
	body := strings.TrimSpace(strings.Join(parts[2:], "---"))
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "source:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "source:")), body
	}
	return "", body
}

// cloneProviderMessages 深拷贝消息切片，避免后台任务读取到后续修改。
func cloneProviderMessages(messages []providertypes.Message) []providertypes.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]providertypes.Message, 0, len(messages))
	for _, message := range messages {
		cloned = append(cloned, cloneProviderMessage(message))
	}
	return cloned
}

// resetTimer 以安全方式重置定时器，避免旧事件污染新的防抖窗口。
func resetTimer(timer *time.Timer, wait time.Duration) *time.Timer {
	if wait < 0 {
		wait = 0
	}
	if timer == nil {
		return time.NewTimer(wait)
	}
	stopTimer(timer)
	timer.Reset(wait)
	return timer
}

// stopTimer 停止定时器并在必要时清空通道。
func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// logError 统一收敛后台提取日志，避免把错误暴露给主链路。
func (a *AutoExtractor) logError(format string, args ...any) {
	if a != nil && a.logf != nil {
		a.logf(format, args...)
	}
}
