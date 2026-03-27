package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderHelp(width int) string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF")).
		Bold(true).
		Render("NeoCode Help")

	b.WriteString(title)
	b.WriteString("\n\n")

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "显示帮助"},
		{"/pwd | /workspace", "查看当前工作区"},
		{"/apikey <env_name>", "切换 API Key 环境变量"},
		{"/provider <name>", "切换 provider"},
		{"/switch <model>", "切换当前模型"},
		{"/memory", "刷新记忆统计"},
		{"/clear-memory confirm", "清空持久记忆"},
		{"/clear-context", "清空当前会话上下文"},
		{"/exit", "退出"},
	}

	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSuccess)).
		Width(22)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMutedText))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDim))

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorTitle))

	for _, c := range commands {
		b.WriteString(cmdStyle.Render(c.cmd))
		b.WriteString(descStyle.Render(c.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("输入框支持 Enter 发送、Alt+Enter 换行、Tab 切换焦点。"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("主视图和侧栏可独立滚动，代码块支持点击 [Copy] 复制。"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Esc 返回主视图，h 折叠/展开侧栏，] 展开/折叠系统消息。"))

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("按 Esc 或 /help 关闭"))

	return lipgloss.NewStyle().MaxWidth(width).Render(b.String())
}
