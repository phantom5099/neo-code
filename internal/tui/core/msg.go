package core

import "neo-code/internal/tui/services"

type BootstrapLoadedMsg struct {
	Data services.BootstrapData
}

func (BootstrapLoadedMsg) isMsg() {}

type InputHandledMsg struct {
	Result services.InputResult
	Err    error
}

func (InputHandledMsg) isMsg() {}

type TurnResolvedMsg struct {
	Resolution services.TurnResolution
}

func (TurnResolvedMsg) isMsg() {}

type MemoryFeedbackMsg struct {
	Feedback *services.MemoryFeedback
	Err      error
}

func (MemoryFeedbackMsg) isMsg() {}

type StreamChunkMsg struct {
	Content string
}

func (StreamChunkMsg) isMsg() {}

type StreamDoneMsg struct{}

func (StreamDoneMsg) isMsg() {}

type StreamErrorMsg struct {
	Err error
}

func (StreamErrorMsg) isMsg() {}

type ExitMsg struct{}

func (ExitMsg) isMsg() {}

type ShowHelpMsg struct{}

func (ShowHelpMsg) isMsg() {}

type HideHelpMsg struct{}

func (HideHelpMsg) isMsg() {}

type RefreshMemoryMsg struct{}

func (RefreshMemoryMsg) isMsg() {}
