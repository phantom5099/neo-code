package runtime

import "neo-code/internal/subagent"

// defaultSubAgentFactory 返回默认的子代理工厂实例。
func defaultSubAgentFactory() subagent.Factory {
	return subagent.NewWorkerFactory(nil)
}

// SetSubAgentFactory 设置子代理运行时工厂；传入 nil 时回退到默认工厂。
func (s *Service) SetSubAgentFactory(factory subagent.Factory) {
	if s == nil {
		return
	}
	s.subAgentMu.Lock()
	defer s.subAgentMu.Unlock()
	if factory == nil {
		s.subAgentFactory = defaultSubAgentFactory()
		return
	}
	s.subAgentFactory = factory
}

// SubAgentFactory 返回当前 runtime 持有的子代理运行时工厂。
func (s *Service) SubAgentFactory() subagent.Factory {
	if s == nil {
		return defaultSubAgentFactory()
	}
	s.subAgentMu.RLock()
	factory := s.subAgentFactory
	s.subAgentMu.RUnlock()
	if factory != nil {
		return factory
	}

	defaultFactory := defaultSubAgentFactory()
	s.subAgentMu.Lock()
	if s.subAgentFactory == nil {
		s.subAgentFactory = defaultFactory
	}
	factory = s.subAgentFactory
	s.subAgentMu.Unlock()
	if factory != nil {
		return factory
	}
	return defaultFactory
}
