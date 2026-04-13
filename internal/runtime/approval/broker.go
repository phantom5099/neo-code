package approval

import (
	"errors"
	"fmt"
	"sync"
)

type pendingRequest struct {
	resultCh  chan Decision
	submitted bool
}

// Broker 负责管理运行期间待审批请求的生命周期。
type Broker struct {
	mu      sync.Mutex
	nextID  uint64
	pending map[string]*pendingRequest
}

// NewBroker 创建一个空的审批请求 broker。
func NewBroker() *Broker {
	return &Broker{
		pending: make(map[string]*pendingRequest),
	}
}

// Open 注册一个新的待审批请求，并返回 request id 与结果通道。
func (b *Broker) Open() (string, chan Decision, error) {
	if b == nil {
		return "", nil, errors.New("runtime: approval broker is nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	requestID := fmt.Sprintf("perm-%d", b.nextID)
	request := &pendingRequest{
		resultCh: make(chan Decision, 1),
	}
	b.pending[requestID] = request
	return requestID, request.resultCh, nil
}

// Resolve 向指定请求提交审批结果；重复提交会被安全忽略。
func (b *Broker) Resolve(requestID string, decision Decision) error {
	if b == nil {
		return errors.New("runtime: approval broker is nil")
	}

	b.mu.Lock()
	request := b.pending[requestID]
	if request == nil {
		b.mu.Unlock()
		return fmt.Errorf("runtime: permission request %q not found", requestID)
	}
	if request.submitted {
		b.mu.Unlock()
		return nil
	}
	request.submitted = true
	resultCh := request.resultCh
	b.mu.Unlock()

	select {
	case resultCh <- decision:
		return nil
	default:
		return nil
	}
}

// Close 清理指定请求，避免后续误用过期 request id。
func (b *Broker) Close(requestID string) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.pending, requestID)
}
