package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
	Streaming bool
	Error     bool
}

type MessageList struct {
	Messages []Message
	Width    int
}

func (ml MessageList) Render() string {
	return ml.RenderLayout().Content
}

func (ml MessageList) RenderLayout() RenderedChatLayout {
	if len(ml.Messages) == 0 {
		return RenderedChatLayout{}
	}

	width := ml.Width
	if width <= 0 {
		width = 80
	}
	bubbleWidth := width - 6
	if bubbleWidth < 20 {
		bubbleWidth = 20
	}

	var builder strings.Builder
	regions := make([]ClickableRegion, 0)
	row := 0

	for idx, msg := range ml.Messages {
		rendered, msgRegions, lines := renderMessage(idx, msg, width, bubbleWidth, row)
		builder.WriteString(rendered)
		builder.WriteString("\n")
		row += lines + 1
		regions = append(regions, msgRegions...)
	}

	return RenderedChatLayout{
		Content:       builder.String(),
		Regions:       regions,
		ContentHeight: row,
	}
}

func renderMessage(messageIndex int, msg Message, width, bubbleWidth, startRow int) (string, []ClickableRegion, int) {
	timestamp := ""
	if !msg.Timestamp.IsZero() {
		timestamp = msg.Timestamp.Format("15:04")
	}

	switch msg.Role {
	case "user":
		return renderUserMessage(msg.Content, timestamp, width, bubbleWidth)
	case "system":
		return renderSystemMessage(msg.Content, timestamp, width)
	default:
		return renderAssistantMessage(messageIndex, msg, timestamp, bubbleWidth, startRow)
	}
}

func renderUserMessage(content, timestamp string, width, bubbleWidth int) (string, []ClickableRegion, int) {
	var builder strings.Builder
	header := DimStyle.Render(timestamp)
	if header != "" {
		builder.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Right, header))
		builder.WriteString("\n")
	}

	bodyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorText)).
		Background(lipgloss.Color(ColorPanelAlt)).
		Padding(0, 1).
		MaxWidth(bubbleWidth)

	body := bodyStyle.Render(strings.TrimSpace(content))
	builder.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Right, body))
	lines := strings.Count(builder.String(), "\n") + 1
	return builder.String(), nil, lines
}

func renderSystemMessage(content, timestamp string, width int) (string, []ClickableRegion, int) {
	var builder strings.Builder
	line := DimStyle.Copy().Italic(true).Render(strings.TrimSpace(content))
	if timestamp != "" {
		line = line + "  " + DimStyle.Render(timestamp)
	}
	builder.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, line))
	lines := strings.Count(builder.String(), "\n") + 1
	return builder.String(), nil, lines
}

func renderAssistantMessage(messageIndex int, msg Message, timestamp string, width int, startRow int) (string, []ClickableRegion, int) {
	var builder strings.Builder
	regions := make([]ClickableRegion, 0)
	rows := 0

	header := "Neo"
	if timestamp != "" {
		header = header + " · " + timestamp
	}
	if msg.Streaming {
		header = header + " · streaming"
	}
	headerStyle := TitleStyle
	if msg.Error {
		headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
	}
	builder.WriteString(headerStyle.Render(header))
	builder.WriteString("\n")
	rows++

	if msg.Error {
		errorBody := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText)).
			Background(lipgloss.Color(ColorError)).
			Padding(0, 1).
			MaxWidth(width).
			Render(strings.TrimSpace(msg.Content))
		builder.WriteString(errorBody)
		rows += strings.Count(errorBody, "\n") + 1
		return builder.String(), regions, rows
	}

	currentRow := startRow + rows
	blockIndex := 0
	for _, segment := range assistantSegments(msg.Content) {
		rendered, region, consumedRows := renderAssistantSegment(messageIndex, &blockIndex, segment, width, currentRow)
		builder.WriteString(rendered)
		rows += consumedRows
		currentRow += consumedRows
		if region != nil {
			regions = append(regions, *region)
		}
	}

	return strings.TrimRight(builder.String(), "\n"), regions, rows
}

func assistantSegments(content string) []ContentSegment {
	segments := ParseContentSegments(content)
	if len(segments) == 0 {
		return []ContentSegment{{Type: SegmentText, Text: "..."}}
	}
	return segments
}

func renderAssistantSegment(messageIndex int, blockIndex *int, segment ContentSegment, width int, row int) (string, *ClickableRegion, int) {
	if segment.Type == SegmentCodeBlock {
		*blockIndex = *blockIndex + 1
		rendered, codeLang, actionStartCol := RenderCodeBlockLayout(segment, width, CopyActionLabel())
		region := BuildCopyRegion(messageIndex, *blockIndex, row, segment.Code, codeLang, actionStartCol)
		return rendered, &region, strings.Count(strings.TrimRight(rendered, "\n"), "\n") + 1
	}

	rendered := renderTextSegment(segment.Text, width)
	return rendered, nil, strings.Count(strings.TrimRight(rendered, "\n"), "\n") + 1
}

func resolveSegmentLanguage(segment ContentSegment) string {
	codeLang := strings.TrimSpace(segment.Lang)
	if codeLang == "" {
		codeLang = DetectLanguage(segment.Code)
	}
	if codeLang == "" {
		return "text"
	}
	return codeLang
}

func renderTextSegment(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMutedText)).
		MaxWidth(width)

	lines := strings.Split(text, "\n")
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(style.Render(line))
		builder.WriteString("\n")
	}
	return builder.String()
}

func MessageSummary(messages []Message) string {
	return fmt.Sprintf("%d msgs", len(messages))
}
