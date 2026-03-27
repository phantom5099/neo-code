package core

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) streamResponse() tea.Cmd {
	controller := m.controller
	request := m.conversationRequest()
	return func() tea.Msg {
		if controller == nil {
			return StreamErrorMsg{Err: context.Canceled}
		}
		stream, err := controller.StartChat(context.Background(), request)
		if err != nil {
			return StreamErrorMsg{Err: err}
		}
		return ChatStartedMsg{Stream: stream}
	}
}

func (m *Model) streamResponseFromChannel() tea.Cmd {
	if m.streamChan == nil {
		return nil
	}

	return func() tea.Msg {
		chunk, ok := <-m.streamChan
		if !ok {
			return StreamDoneMsg{}
		}
		return StreamChunkMsg{Content: chunk}
	}
}
