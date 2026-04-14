package runtime

import "neo-code/internal/subagent"

// SetSubAgentFactory 设置子代理运行时工厂；传入 nil 时回退到默认工厂。
func (s *Service) SetSubAgentFactory(factory subagent.Factory) {
	if factory == nil {
		s.subAgentFactory = subagent.NewWorkerFactory(nil)
		return
	}
	s.subAgentFactory = factory
}

// SubAgentFactory 返回当前 runtime 持有的子代理运行时工厂。
func (s *Service) SubAgentFactory() subagent.Factory {
	if s.subAgentFactory == nil {
		s.subAgentFactory = subagent.NewWorkerFactory(nil)
	}
	return s.subAgentFactory
}
