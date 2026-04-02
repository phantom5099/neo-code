package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

type copyCodeButtonBinding struct {
	ID   int
	Code string
}

var (
	copyCodeButtonPattern = regexp.MustCompile(`\[Copy code #([0-9]+)\]`)
	clipboardWriteAll     = clipboard.WriteAll
)

func collectCopyCodeButtons(content string, startID int) []copyCodeButtonBinding {
	codeBlocks := extractFencedCodeBlocks(content)
	if len(codeBlocks) == 0 {
		return nil
	}

	bindings := make([]copyCodeButtonBinding, 0, len(codeBlocks))
	for i, code := range codeBlocks {
		bindings = append(bindings, copyCodeButtonBinding{
			ID:   startID + i,
			Code: code,
		})
	}
	return bindings
}

func extractFencedCodeBlocks(content string) []string {
	parts := strings.Split(content, "```")
	if len(parts) < 3 {
		return nil
	}

	blocks := make([]string, 0, len(parts)/2)
	for i := 1; i < len(parts); i += 2 {
		code := strings.Trim(parts[i], "\n")
		if code == "" {
			continue
		}
		lines := strings.Split(code, "\n")
		if len(lines) > 1 && !strings.Contains(lines[0], " ") && !strings.Contains(lines[0], "\t") {
			code = strings.Join(lines[1:], "\n")
		}
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		blocks = append(blocks, code)
	}
	return blocks
}

func (a *App) setCodeCopyBlocks(bindings []copyCodeButtonBinding) {
	a.codeCopyBlocks = make(map[int]string, len(bindings))
	for _, binding := range bindings {
		a.codeCopyBlocks[binding.ID] = binding.Code
	}
}

func parseCopyCodeButtonID(line string) (int, bool) {
	clean := ansiEscapePattern.ReplaceAllString(line, "")
	matches := copyCodeButtonPattern.FindStringSubmatch(clean)
	if len(matches) != 2 {
		return 0, false
	}

	id, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return id, true
}

func (a *App) handleTranscriptCopyClick(msg tea.MouseMsg) bool {
	line, ok := a.transcriptLineAtMouse(msg)
	if !ok {
		return false
	}

	buttonID, ok := parseCopyCodeButtonID(line)
	if !ok {
		return false
	}

	code, ok := a.codeCopyBlocks[buttonID]
	if !ok {
		a.state.ExecutionError = statusCodeCopyError
		a.state.StatusText = statusCodeCopyError
		a.appendActivity("clipboard", statusCodeCopyError, fmt.Sprintf("button #%d", buttonID), true)
		return true
	}

	if err := clipboardWriteAll(code); err != nil {
		a.state.ExecutionError = err.Error()
		a.state.StatusText = statusCodeCopyError
		a.appendActivity("clipboard", statusCodeCopyError, err.Error(), true)
		return true
	}

	a.state.ExecutionError = ""
	a.state.StatusText = fmt.Sprintf(statusCodeCopied, buttonID)
	a.appendActivity("clipboard", "Copied code block", fmt.Sprintf("#%d", buttonID), false)
	return true
}

func (a App) transcriptLineAtMouse(msg tea.MouseMsg) (string, bool) {
	if !a.isMouseWithinTranscript(msg) {
		return "", false
	}

	_, y, _, _ := a.transcriptBounds()
	lineIndex := msg.Y - y
	if lineIndex < 0 {
		return "", false
	}

	lines := strings.Split(a.transcript.View(), "\n")
	if lineIndex >= len(lines) {
		return "", false
	}
	return lines[lineIndex], true
}
