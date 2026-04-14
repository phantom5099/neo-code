package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/tools"
)

// executeAssistantToolCalls 并发执行 assistant 返回的全部工具调用并回写结果。
func (s *Service) executeAssistantToolCalls(
	ctx context.Context,
	state *runState,
	snapshot turnSnapshot,
	assistant providertypes.Message,
) error {
	if len(assistant.ToolCalls) == 0 {
		return nil
	}

	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	orderedCalls := reorderToolCallsByNameRoundRobin(assistant.ToolCalls)
	readyCalls, skippedCalls := splitDuplicateToolCallsInTurn(orderedCalls)
	for _, skipped := range skippedCalls {
		result := newDuplicateToolCallResult(skipped.call.Name, skipped.call.ID, skipped.originalCallID)
		if err := s.appendToolMessageAndSave(execCtx, state, skipped.call, result); err != nil {
			return err
		}
		s.emitRunScoped(execCtx, EventToolResult, state, result)
	}
	if len(readyCalls) == 0 {
		return nil
	}

	parallelism := resolveToolParallelism(len(readyCalls))
	toolLocks := buildToolExecutionLocks(readyCalls)
	taskCh := make(chan providertypes.ToolCall)
	var mu sync.Mutex
	var firstErr error
	var workerWG sync.WaitGroup

	checkContext := func() bool {
		return shouldStopToolExecution(&mu, &firstErr, execCtx.Err())
	}

	for i := 0; i < parallelism; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for call := range taskCh {
				s.executeOneToolCall(
					execCtx,
					state,
					snapshot,
					call,
					toolLocks[normalizeToolLockKey(call.Name)],
					checkContext,
					func(err error) {
						recordAndCancelOnFirstError(&mu, &firstErr, err, cancelExec)
					},
				)
			}
		}()
	}

	for _, call := range readyCalls {
		if checkContext() {
			break
		}
		taskCh <- call
	}

	close(taskCh)
	workerWG.Wait()
	return firstErr
}

// executeOneToolCall 在单个 worker 中执行一次工具调用并处理结果回写与事件发射。
func (s *Service) executeOneToolCall(
	ctx context.Context,
	state *runState,
	snapshot turnSnapshot,
	call providertypes.ToolCall,
	toolLock *sync.Mutex,
	checkContext func() bool,
	rememberError func(error),
) {
	if checkContext() {
		return
	}

	toolLock.Lock()
	defer toolLock.Unlock()

	s.emitRunScoped(ctx, EventToolStart, state, call)

	result, execErr := s.executeToolCallWithPermission(ctx, permissionExecutionInput{
		RunID:       state.runID,
		SessionID:   state.session.ID,
		Call:        call,
		Workdir:     snapshot.workdir,
		ToolTimeout: snapshot.toolTimeout,
	})

	if errors.Is(execErr, context.Canceled) {
		rememberError(execErr)
		return
	}
	if execErr == nil && checkContext() {
		return
	}

	if execErr != nil && strings.TrimSpace(result.Content) == "" {
		result.Content = execErr.Error()
	}

	if err := s.appendToolMessageAndSave(ctx, state, call, result); err != nil {
		if execErr != nil && errors.Is(err, context.Canceled) {
			s.emitRunScoped(ctx, EventToolResult, state, result)
		}
		rememberError(err)
		return
	}

	if execErr == nil && checkContext() {
		return
	}

	s.emitRunScoped(ctx, EventToolResult, state, result)

	if isSuccessfulRememberToolCall(call.Name, result, execErr) {
		state.mu.Lock()
		state.rememberedThisRun = true
		state.mu.Unlock()
	}

	if execErr != nil && checkContext() {
		return
	}
}

// resolveToolParallelism 计算本轮工具执行的并发上限，避免无界 goroutine 扩散。
func resolveToolParallelism(toolCallCount int) int {
	if toolCallCount <= 0 {
		return 1
	}
	if toolCallCount < defaultToolParallelism {
		return toolCallCount
	}
	return defaultToolParallelism
}

// reorderToolCallsByNameRoundRobin 按工具名分组后轮询展开，降低同名批量调用导致的队头阻塞。
func reorderToolCallsByNameRoundRobin(calls []providertypes.ToolCall) []providertypes.ToolCall {
	if len(calls) <= 1 {
		return append([]providertypes.ToolCall(nil), calls...)
	}
	grouped := make(map[string][]providertypes.ToolCall, len(calls))
	order := make([]string, 0, len(calls))
	for _, call := range calls {
		key := normalizeToolLockKey(call.Name)
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], call)
	}

	ordered := make([]providertypes.ToolCall, 0, len(calls))
	for {
		progressed := false
		for _, key := range order {
			queue := grouped[key]
			if len(queue) == 0 {
				continue
			}
			ordered = append(ordered, queue[0])
			grouped[key] = queue[1:]
			progressed = true
		}
		if !progressed {
			break
		}
	}
	return ordered
}

// buildToolExecutionLocks 按工具名构造互斥锁，确保同名工具调用在单轮内串行执行。
func buildToolExecutionLocks(calls []providertypes.ToolCall) map[string]*sync.Mutex {
	locks := make(map[string]*sync.Mutex, len(calls))
	for _, call := range calls {
		key := normalizeToolLockKey(call.Name)
		if _, exists := locks[key]; !exists {
			locks[key] = &sync.Mutex{}
		}
	}
	return locks
}

// normalizeToolLockKey 将工具名规范化为锁键，防止大小写差异导致重复并发执行。
func normalizeToolLockKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type duplicateToolCall struct {
	call           providertypes.ToolCall
	originalCallID string
}

const toolCallFingerprintSeparator = "\x1f"

// splitDuplicateToolCallsInTurn 在单轮内按工具名+参数指纹去重，并返回被跳过的重复调用。
func splitDuplicateToolCallsInTurn(calls []providertypes.ToolCall) ([]providertypes.ToolCall, []duplicateToolCall) {
	if len(calls) <= 1 {
		return append([]providertypes.ToolCall(nil), calls...), nil
	}

	uniqueCalls := make([]providertypes.ToolCall, 0, len(calls))
	skippedCalls := make([]duplicateToolCall, 0, len(calls))
	seen := make(map[string]string, len(calls))
	for _, call := range calls {
		fingerprint := buildToolCallFingerprint(call)
		if firstCallID, exists := seen[fingerprint]; exists {
			skippedCalls = append(skippedCalls, duplicateToolCall{
				call:           call,
				originalCallID: firstCallID,
			})
			continue
		}
		seen[fingerprint] = strings.TrimSpace(call.ID)
		uniqueCalls = append(uniqueCalls, call)
	}
	return uniqueCalls, skippedCalls
}

// buildToolCallFingerprint 生成工具调用指纹，用于识别同轮内 name+args 完全重复的调用。
func buildToolCallFingerprint(call providertypes.ToolCall) string {
	return normalizeToolLockKey(call.Name) + toolCallFingerprintSeparator + canonicalizeToolCallArguments(call.Arguments)
}

// canonicalizeToolCallArguments 将参数规范化为稳定字符串，JSON 解析失败时回退为裁剪后的原始文本。
func canonicalizeToolCallArguments(rawArgs string) string {
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" {
		return ""
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return trimmed
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return trimmed
	}
	return string(encoded)
}

// newDuplicateToolCallResult 生成重复调用的统一错误结果，显式携带被复用的首个 call_id。
func newDuplicateToolCallResult(toolName string, duplicateCallID string, originalCallID string) tools.ToolResult {
	duplicateCallID = strings.TrimSpace(duplicateCallID)
	originalCallID = strings.TrimSpace(originalCallID)
	details := fmt.Sprintf("call_id=%s duplicates call_id=%s", duplicateCallID, originalCallID)
	return tools.NewErrorResult(toolName, "duplicate_tool_call", details, nil)
}

// rememberFirstError 记录首次错误，后续错误只保留用于日志和事件路径。
func rememberFirstError(mu *sync.Mutex, firstErr *error, err error) bool {
	if err == nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	if *firstErr == nil {
		*firstErr = err
		return true
	}
	return false
}

// shouldStopToolExecution 统一判断工具执行是否应停止，并在上下文取消时兜底记录错误原因。
func shouldStopToolExecution(mu *sync.Mutex, firstErr *error, contextErr error) bool {
	mu.Lock()
	defer mu.Unlock()
	if contextErr != nil && *firstErr == nil {
		*firstErr = contextErr
	}
	return *firstErr != nil
}

// recordAndCancelOnFirstError 在首次记录错误时触发执行上下文取消，阻止后续工具继续派发。
func recordAndCancelOnFirstError(mu *sync.Mutex, firstErr *error, err error, cancel context.CancelFunc) {
	if rememberFirstError(mu, firstErr, err) {
		cancel()
	}
}
