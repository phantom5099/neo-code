package tui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dust/neo-code/internal/config"
	"github.com/dust/neo-code/internal/provider"
	agentruntime "github.com/dust/neo-code/internal/runtime"
	"github.com/dust/neo-code/internal/tools"
)

type RuntimeMsg struct{ Event agentruntime.RuntimeEvent }
type RuntimeClosedMsg struct{}
type runFinishedMsg struct{ err error }
type localCommandResultMsg struct {
	notice string
	err    error
}
type slashCommand struct {
	Usage       string
	Description string
}
type commandSuggestion struct {
	Command slashCommand
	Match   bool
}
type layout struct {
	stacked       bool
	contentWidth  int
	contentHeight int
	sidebarWidth  int
	sidebarHeight int
	rightWidth    int
	rightHeight   int
}

type App struct {
	state          UIState
	configManager  *config.Manager
	runtime        agentruntime.Runtime
	keys           keyMap
	help           help.Model
	spinner        spinner.Model
	sessions       list.Model
	modelPicker    list.Model
	transcript     viewport.Model
	input          textarea.Model
	activeMessages []provider.Message
	focus          panel
	width          int
	height         int
	styles         styles
}

func New(cfg *config.Config, configManager *config.Manager, runtime agentruntime.Runtime) (App, error) {
	if cfg == nil {
		defaultCfg := config.Default()
		cfg = defaultCfg
	}
	if configManager == nil {
		return App{}, fmt.Errorf("tui: config manager is nil")
	}

	uiStyles := newStyles()
	keys := newKeyMap()
	delegate := sessionDelegate{styles: uiStyles}
	sessionList := list.New([]list.Item{}, delegate, 0, 0)
	sessionList.Title = ""
	sessionList.SetShowHelp(false)
	sessionList.SetShowStatusBar(false)
	sessionList.SetFilteringEnabled(true)
	sessionList.DisableQuitKeybindings()

	input := textarea.New()
	input.Placeholder = "Ask NeoCode to inspect, edit, or build. Type / to browse commands."
	input.Prompt = "> "
	input.CharLimit = 24000
	input.ShowLineNumbers = false
	input.SetHeight(1)
	input.Focus()
	input.KeyMap.InsertNewline.SetEnabled(false)

	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary))

	h := help.New()
	h.ShowAll = false

	app := App{
		state: UIState{
			StatusText:         "Ready",
			CurrentProvider:    cfg.SelectedProvider,
			CurrentModel:       cfg.CurrentModel,
			CurrentWorkdir:     cfg.Workdir,
			ActiveSessionTitle: "Draft",
			Focus:              panelInput,
		},
		configManager: configManager,
		runtime:       runtime,
		keys:          keys,
		help:          h,
		spinner:       spin,
		sessions:      sessionList,
		modelPicker:   newModelPicker(),
		transcript:    viewport.New(0, 0),
		input:         input,
		focus:         panelInput,
		width:         128,
		height:        40,
		styles:        uiStyles,
	}

	if err := app.refreshSessions(); err != nil {
		return App{}, err
	}
	if len(app.state.Sessions) > 0 {
		app.state.ActiveSessionID = app.state.Sessions[0].ID
		if err := app.refreshMessages(); err != nil {
			return App{}, err
		}
	}
	app.syncActiveSessionTitle()
	app.syncConfigState(configManager.Get())
	app.selectCurrentModel(cfg.CurrentModel)
	app.resizeComponents()
	return app, nil
}

func newModelPicker() list.Model {
	items := make([]list.Item, 0, len(defaultModelItems()))
	for _, item := range defaultModelItems() {
		items = append(items, item)
	}
	delegate := list.NewDefaultDelegate()
	picker := list.New(items, delegate, 0, 0)
	picker.Title = ""
	picker.SetShowHelp(false)
	picker.SetShowStatusBar(false)
	picker.SetFilteringEnabled(false)
	picker.DisableQuitKeybindings()
	return picker
}

func defaultModelItems() []modelItem {
	return []modelItem{
		{name: "gpt-4o", description: "General-purpose OpenAI model"},
		{name: "gpt-4.5-preview", description: "Preview reasoning and writing model"},
		{name: "gpt-5.4", description: "Latest frontier general model"},
		{name: "gpt-5.3-codex", description: "Code-focused GPT-5.3 variant"},
		{name: "claude-3-7-sonnet-latest", description: "Anthropic balanced coding model"},
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(ListenForRuntimeEvent(a.runtime.Events()), textarea.Blink, a.spinner.Tick)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var spinCmd tea.Cmd
	a.spinner, spinCmd = a.spinner.Update(msg)
	if a.state.IsAgentRunning {
		cmds = append(cmds, spinCmd)
	}

	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = typed.Width
		a.height = typed.Height
		a.resizeComponents()
		return a, tea.Batch(cmds...)
	case RuntimeMsg:
		a.handleRuntimeEvent(typed.Event)
		_ = a.refreshSessions()
		a.syncActiveSessionTitle()
		a.rebuildTranscript()
		cmds = append(cmds, ListenForRuntimeEvent(a.runtime.Events()))
		return a, tea.Batch(cmds...)
	case RuntimeClosedMsg:
		a.state.IsAgentRunning = false
		if strings.TrimSpace(a.state.StatusText) == "" {
			a.state.StatusText = "Runtime closed"
		}
		a.rebuildTranscript()
		return a, tea.Batch(cmds...)
	case runFinishedMsg:
		if typed.err != nil {
			a.state.IsAgentRunning = false
			a.state.StreamingReply = false
			a.state.CurrentTool = ""
			a.state.ExecutionError = typed.err.Error()
			a.state.StatusText = typed.err.Error()
			a.appendInlineMessage("error", typed.err.Error())
		}
		_ = a.refreshSessions()
		a.syncActiveSessionTitle()
		a.rebuildTranscript()
		return a, tea.Batch(cmds...)
	case localCommandResultMsg:
		if typed.err != nil {
			a.state.ExecutionError = typed.err.Error()
			a.state.StatusText = typed.err.Error()
			a.appendInlineMessage("error", typed.err.Error())
		} else {
			a.state.ExecutionError = ""
			a.state.StatusText = typed.notice
			cfg := a.configManager.Get()
			a.syncConfigState(cfg)
			a.selectCurrentModel(cfg.CurrentModel)
			a.appendInlineMessage("system", typed.notice)
		}
		a.rebuildTranscript()
		return a, tea.Batch(cmds...)
	case tea.KeyMsg:
		if key.Matches(typed, a.keys.Quit) {
			return a, tea.Quit
		}
		if key.Matches(typed, a.keys.ToggleHelp) {
			a.state.ShowHelp = !a.state.ShowHelp
			a.help.ShowAll = a.state.ShowHelp
			a.resizeComponents()
			return a, tea.Batch(cmds...)
		}
		if a.state.ShowModelPicker {
			return a.updateModelPicker(typed)
		}
		if key.Matches(typed, a.keys.NextPanel) {
			a.focusNext()
			return a, tea.Batch(cmds...)
		}
		if key.Matches(typed, a.keys.PrevPanel) {
			a.focusPrev()
			return a, tea.Batch(cmds...)
		}
		if key.Matches(typed, a.keys.FocusInput) {
			a.focus = panelInput
			a.applyFocus()
			return a, tea.Batch(cmds...)
		}
		if key.Matches(typed, a.keys.NewSession) && !a.state.IsAgentRunning {
			a.state.ActiveSessionID = ""
			a.state.ActiveSessionTitle = "Draft"
			a.activeMessages = nil
			a.state.StatusText = "New draft"
			a.state.ExecutionError = ""
			a.state.CurrentTool = ""
			a.input.Reset()
			a.state.InputText = ""
			a.focus = panelInput
			a.applyFocus()
			a.rebuildTranscript()
			return a, tea.Batch(cmds...)
		}
		switch a.focus {
		case panelSessions:
			if key.Matches(typed, a.keys.OpenSession) && !a.isFilteringSessions() {
				if err := a.activateSelectedSession(); err != nil {
					a.state.StatusText = err.Error()
					a.state.ExecutionError = err.Error()
					a.appendInlineMessage("error", err.Error())
				}
				a.focus = panelInput
				a.applyFocus()
				a.rebuildTranscript()
				return a, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			a.sessions, cmd = a.sessions.Update(msg)
			cmds = append(cmds, cmd)
			return a, tea.Batch(cmds...)
		case panelTranscript:
			a.handleViewportKeys(&a.transcript, typed)
			return a, tea.Batch(cmds...)
		case panelInput:
			if key.Matches(typed, a.keys.Send) {
				input := strings.TrimSpace(a.input.Value())
				if input == "" || a.state.IsAgentRunning {
					return a, tea.Batch(cmds...)
				}
				a.input.Reset()
				a.state.InputText = ""
				switch strings.ToLower(input) {
				case "/model", "/set model":
					a.openModelPicker()
					return a, tea.Batch(cmds...)
				}
				if strings.HasPrefix(input, "/") {
					a.state.StatusText = "Applying local command"
					cmds = append(cmds, runLocalCommand(a.configManager, input))
					return a, tea.Batch(cmds...)
				}
				a.state.IsAgentRunning = true
				a.state.StreamingReply = false
				a.state.ExecutionError = ""
				a.state.StatusText = "Thinking"
				a.state.CurrentTool = ""
				a.activeMessages = append(a.activeMessages, provider.Message{Role: "user", Content: input})
				a.rebuildTranscript()
				cmds = append(cmds, runAgent(a.runtime, a.state.ActiveSessionID, input))
				return a, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			a.state.InputText = a.input.Value()
			a.resizeComponents()
			cmds = append(cmds, cmd)
			return a, tea.Batch(cmds...)
		}
	}
	return a, tea.Batch(cmds...)
}

func (a App) updateModelPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.FocusInput):
		a.closeModelPicker()
		return a, nil
	case msg.String() == "enter":
		item, ok := a.modelPicker.SelectedItem().(modelItem)
		a.closeModelPicker()
		if !ok {
			return a, nil
		}
		return a, runModelSelection(a.configManager, item.name)
	}
	var cmd tea.Cmd
	a.modelPicker, cmd = a.modelPicker.Update(msg)
	return a, cmd
}

func (a *App) openModelPicker() {
	a.state.ShowModelPicker = true
	a.state.StatusText = "Choose a model"
	a.input.Blur()
	a.selectCurrentModel(a.state.CurrentModel)
}

func (a *App) closeModelPicker() {
	a.state.ShowModelPicker = false
	a.focus = panelInput
	a.applyFocus()
}

func (a *App) selectCurrentModel(model string) {
	items := a.modelPicker.Items()
	for idx, item := range items {
		candidate, ok := item.(modelItem)
		if ok && strings.EqualFold(candidate.name, model) {
			a.modelPicker.Select(idx)
			return
		}
	}
	if len(items) > 0 {
		a.modelPicker.Select(0)
	}
}

func (a App) View() string {
	if a.width < 84 || a.height < 24 {
		return a.styles.doc.Render("Window too small.\nPlease resize to at least 84x24.")
	}
	lay := a.computeLayout()
	header := a.renderHeader()
	body := a.renderBody(lay)
	helpView := a.renderHelp(lay.contentWidth)
	return a.styles.doc.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, helpView))
}

func (a *App) refreshSessions() error {
	sessions, err := a.runtime.ListSessions(context.Background())
	if err != nil {
		return err
	}
	a.state.Sessions = sessions
	var selectedID string
	if item, ok := a.sessions.SelectedItem().(sessionItem); ok {
		selectedID = item.Summary.ID
	}
	items := make([]list.Item, 0, len(sessions))
	cursor := 0
	for i, summary := range sessions {
		items = append(items, sessionItem{Summary: summary, Active: summary.ID == a.state.ActiveSessionID})
		if summary.ID == selectedID || summary.ID == a.state.ActiveSessionID {
			cursor = i
		}
	}
	a.sessions.SetItems(items)
	if len(items) > 0 {
		a.sessions.Select(cursor)
	}
	return nil
}

func (a *App) refreshMessages() error {
	if strings.TrimSpace(a.state.ActiveSessionID) == "" {
		a.activeMessages = nil
		return nil
	}
	session, err := a.runtime.LoadSession(context.Background(), a.state.ActiveSessionID)
	if err != nil {
		return err
	}
	a.activeMessages = session.Messages
	a.state.ActiveSessionTitle = session.Title
	return nil
}

func (a *App) activateSelectedSession() error {
	item, ok := a.sessions.SelectedItem().(sessionItem)
	if !ok {
		return nil
	}
	a.state.ActiveSessionID = item.Summary.ID
	a.state.ActiveSessionTitle = item.Summary.Title
	a.state.ExecutionError = ""
	a.state.CurrentTool = ""
	if err := a.refreshSessions(); err != nil {
		return err
	}
	return a.refreshMessages()
}

func (a *App) syncActiveSessionTitle() {
	if strings.TrimSpace(a.state.ActiveSessionID) == "" {
		if strings.TrimSpace(a.state.ActiveSessionTitle) == "" {
			a.state.ActiveSessionTitle = "Draft"
		}
		return
	}
	for _, item := range a.state.Sessions {
		if item.ID == a.state.ActiveSessionID {
			a.state.ActiveSessionTitle = item.Title
			return
		}
	}
}

func (a *App) syncConfigState(cfg config.Config) {
	a.state.CurrentProvider = cfg.SelectedProvider
	a.state.CurrentModel = cfg.CurrentModel
	a.state.CurrentWorkdir = cfg.Workdir
}

func (a *App) handleRuntimeEvent(event agentruntime.RuntimeEvent) {
	if a.state.ActiveSessionID == "" {
		a.state.ActiveSessionID = event.SessionID
	}
	switch event.Type {
	case agentruntime.EventUserMessage:
		a.state.StatusText = "Thinking"
		a.state.StreamingReply = false
		a.state.CurrentTool = ""
		a.state.ExecutionError = ""
	case agentruntime.EventToolStart:
		a.state.StatusText = "Running tool"
		a.state.StreamingReply = false
		if payload, ok := event.Payload.(provider.ToolCall); ok {
			a.state.CurrentTool = payload.Name
			a.appendInlineMessage("event", "Running tool: "+payload.Name+"...")
		}
	case agentruntime.EventToolResult:
		a.state.StreamingReply = false
		a.state.CurrentTool = ""
		if payload, ok := event.Payload.(tools.ToolResult); ok {
			if payload.IsError {
				a.state.ExecutionError = payload.Content
				a.state.StatusText = "Tool error"
				a.appendInlineMessage("error", preview(payload.Content, 88, 4))
			} else if strings.TrimSpace(a.state.ExecutionError) == "" {
				a.state.StatusText = "Tool finished"
				a.appendInlineMessage("event", "Completed tool: "+payload.Name)
			}
		}
	case agentruntime.EventAgentChunk:
		if payload, ok := event.Payload.(string); ok {
			a.appendAssistantChunk(payload)
		}
	case agentruntime.EventAgentDone:
		a.state.IsAgentRunning = false
		a.state.StreamingReply = false
		a.state.CurrentTool = ""
		if strings.TrimSpace(a.state.ExecutionError) == "" {
			a.state.StatusText = "Ready"
		}
		if payload, ok := event.Payload.(provider.Message); ok && strings.TrimSpace(payload.Content) != "" && !a.lastAssistantMatches(payload.Content) {
			a.activeMessages = append(a.activeMessages, provider.Message{Role: "assistant", Content: payload.Content})
		}
	case agentruntime.EventError:
		a.state.StatusText = "Error"
		a.state.IsAgentRunning = false
		a.state.StreamingReply = false
		a.state.CurrentTool = ""
		if payload, ok := event.Payload.(string); ok {
			a.state.ExecutionError = payload
			a.state.StatusText = payload
			a.appendInlineMessage("error", payload)
		}
	}
}

func (a *App) appendAssistantChunk(chunk string) {
	if chunk == "" {
		return
	}
	if !a.state.StreamingReply || len(a.activeMessages) == 0 || a.activeMessages[len(a.activeMessages)-1].Role != "assistant" {
		a.activeMessages = append(a.activeMessages, provider.Message{Role: "assistant", Content: chunk})
		a.state.StreamingReply = true
		return
	}
	a.activeMessages[len(a.activeMessages)-1].Content += chunk
}

func (a *App) appendInlineMessage(role string, message string) {
	content := strings.TrimSpace(message)
	if content == "" {
		return
	}
	a.activeMessages = append(a.activeMessages, provider.Message{Role: role, Content: content})
}

func (a *App) lastAssistantMatches(content string) bool {
	if len(a.activeMessages) == 0 {
		return false
	}
	last := a.activeMessages[len(a.activeMessages)-1]
	return last.Role == "assistant" && strings.TrimSpace(last.Content) == strings.TrimSpace(content)
}

func (a *App) handleViewportKeys(vp *viewport.Model, msg tea.KeyMsg) {
	switch {
	case key.Matches(msg, a.keys.ScrollUp):
		vp.LineUp(2)
	case key.Matches(msg, a.keys.ScrollDown):
		vp.LineDown(2)
	case key.Matches(msg, a.keys.PageUp):
		vp.HalfViewUp()
	case key.Matches(msg, a.keys.PageDown):
		vp.HalfViewDown()
	case key.Matches(msg, a.keys.Top):
		vp.GotoTop()
	case key.Matches(msg, a.keys.Bottom):
		vp.GotoBottom()
	}
}

func (a *App) focusNext() {
	order := []panel{panelSessions, panelTranscript, panelInput}
	current := 0
	for i, item := range order {
		if item == a.focus {
			current = i
			break
		}
	}
	a.focus = order[(current+1)%len(order)]
	a.applyFocus()
}

func (a *App) focusPrev() {
	order := []panel{panelSessions, panelTranscript, panelInput}
	current := 0
	for i, item := range order {
		if item == a.focus {
			current = i
			break
		}
	}
	if current == 0 {
		a.focus = order[len(order)-1]
	} else {
		a.focus = order[current-1]
	}
	a.applyFocus()
}

func (a *App) applyFocus() {
	a.state.Focus = a.focus
	if a.focus == panelInput && !a.state.ShowModelPicker {
		a.input.Focus()
		return
	}
	a.input.Blur()
}

func (a *App) resizeComponents() {
	lay := a.computeLayout()
	a.help.ShowAll = a.state.ShowHelp
	a.sessions.SetSize(max(20, lay.sidebarWidth-4), max(4, lay.sidebarHeight-4))
	menuHeight := a.commandMenuHeight(max(24, lay.rightWidth))
	a.transcript.Width = max(24, lay.rightWidth)
	a.transcript.Height = max(6, lay.rightHeight-2-menuHeight-1)
	a.input.SetWidth(max(24, lay.rightWidth-2))
	a.input.SetHeight(1)
	a.modelPicker.SetSize(max(24, clamp(lay.rightWidth-14, 28, 52)), max(4, clamp(lay.rightHeight-10, 6, 10)))
	a.rebuildTranscript()
}

func (a *App) rebuildTranscript() {
	width := max(24, a.transcript.Width)
	if len(a.activeMessages) == 0 {
		a.transcript.SetContent(a.styles.empty.Width(width).Render("No conversation yet.\nAsk NeoCode to inspect or change code, or type / to browse local commands."))
		a.transcript.GotoTop()
		return
	}
	atBottom := a.transcript.AtBottom()
	blocks := make([]string, 0, len(a.activeMessages))
	for _, message := range a.activeMessages {
		blocks = append(blocks, a.renderMessageBlock(message, width))
	}
	a.transcript.SetContent(strings.Join(blocks, "\n\n"))
	if atBottom || a.state.IsAgentRunning {
		a.transcript.GotoBottom()
	}
}

func (a App) renderHeader() string {
	status := a.state.StatusText
	if a.state.IsAgentRunning {
		status = a.spinner.View() + " " + fallback(status, "Running")
	}
	brand := lipgloss.JoinHorizontal(lipgloss.Center, a.styles.headerBrand.Render("NeoCode"), a.styles.headerSpacer.Render(""), a.styles.headerSub.Render("immersive coding agent"))
	meta := lipgloss.JoinHorizontal(lipgloss.Top, a.styles.badgeAgent.Render("Provider "+a.state.CurrentProvider), a.styles.badgeUser.Render("Model "+a.state.CurrentModel), a.styles.badgeMuted.Render("Focus "+a.focusLabel()), a.statusBadge(status))
	return lipgloss.JoinVertical(lipgloss.Left, brand, lipgloss.JoinHorizontal(lipgloss.Top, a.styles.headerMeta.Render("Workdir "+trimMiddle(a.state.CurrentWorkdir, max(28, a.width/3))), a.styles.headerSpacer.Render(""), meta))
}

func (a App) renderBody(lay layout) string {
	sidebar := a.renderSidebar(lay.sidebarWidth, lay.sidebarHeight)
	stream := a.renderWaterfall(lay.rightWidth, lay.rightHeight)
	if lay.stacked {
		return lipgloss.JoinVertical(lipgloss.Left, sidebar, stream)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, stream)
}

func (a App) renderSidebar(width int, height int) string {
	return a.renderPanel("Sessions", "Use / to filter, Enter to open", a.sessions.View(), width, height, a.focus == panelSessions)
}

func (a App) renderWaterfall(width int, height int) string {
	if a.state.ShowModelPicker {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, a.renderModelPicker(clamp(width-10, 36, 56), clamp(height-6, 10, 14)))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, a.styles.streamTitle.Render(fallback(a.state.ActiveSessionTitle, "Draft")), a.styles.headerSpacer.Render(""), a.styles.streamMeta.Render(fmt.Sprintf("%d messages", len(a.activeMessages))))
	subline := lipgloss.JoinHorizontal(lipgloss.Top, a.styles.streamMeta.Render("Active model "+a.state.CurrentModel), a.styles.headerSpacer.Render(""), a.styles.streamMeta.Render(fallback(a.state.CurrentTool, a.state.StatusText)))
	transcript := a.styles.streamContent.Width(width).Height(a.transcript.Height).Render(a.transcript.View())
	parts := []string{header, subline, transcript}
	if menu := a.renderCommandMenu(width); menu != "" {
		parts = append(parts, menu)
	}
	parts = append(parts, a.renderPrompt(width))
	return lipgloss.NewStyle().Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (a App) renderModelPicker(width int, height int) string {
	content := lipgloss.JoinVertical(lipgloss.Left, a.styles.panelTitle.Render("Select Model"), a.styles.panelSubtitle.Render("Up/Down choose, Enter confirm, Esc cancel"), a.modelPicker.View())
	return a.styles.panelFocused.Width(width).Height(height).Render(content)
}

func (a App) renderPrompt(width int) string {
	return lipgloss.JoinVertical(lipgloss.Left, a.styles.inputMeta.Render("Enter send | /set url | /set key | /model | Tab switch panels"), a.styles.inputLine.Width(width).Render(a.input.View()))
}

func (a App) renderCommandMenu(width int) string {
	suggestions := a.matchingSlashCommands(strings.TrimSpace(a.input.Value()))
	if len(suggestions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(suggestions)+1)
	lines = append(lines, a.styles.commandMenuTitle.Render("Commands"))
	for _, suggestion := range suggestions {
		usageStyle := a.styles.commandUsage
		if suggestion.Match {
			usageStyle = a.styles.commandUsageMatch
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, usageStyle.Render(suggestion.Command.Usage), lipgloss.NewStyle().Width(2).Render(""), a.styles.commandDesc.Render(suggestion.Command.Description)))
	}
	return a.styles.commandMenu.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (a App) matchingSlashCommands(input string) []commandSuggestion {
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(input))
	all := []slashCommand{
		{Usage: "/set url <url>", Description: "Set the API Base URL"},
		{Usage: "/set key <key>", Description: "Update the API Key"},
		{Usage: "/model", Description: "Open the interactive model picker"},
	}
	out := make([]commandSuggestion, 0, len(all))
	for _, command := range all {
		normalized := strings.ToLower(command.Usage)
		match := query == "/" || strings.HasPrefix(normalized, query)
		if query == "/" || match || strings.Contains(normalized, query) {
			out = append(out, commandSuggestion{Command: command, Match: match})
		}
	}
	return out
}

func (a App) commandMenuHeight(width int) int {
	menu := a.renderCommandMenu(width)
	if strings.TrimSpace(menu) == "" {
		return 0
	}
	return lipgloss.Height(menu)
}

func (a App) renderHelp(width int) string {
	a.help.ShowAll = a.state.ShowHelp
	return a.styles.footer.Width(width).Render(a.help.View(a.keys))
}

func (a App) renderPanel(title string, subtitle string, body string, width int, height int, focused bool) string {
	style := a.styles.panel
	if focused {
		style = a.styles.panelFocused
	}
	header := lipgloss.JoinHorizontal(lipgloss.Center, a.styles.panelTitle.Render(title), lipgloss.NewStyle().Width(2).Render(""), a.styles.panelSubtitle.Render(subtitle))
	bodyHeight := max(3, height-lipgloss.Height(header)-2)
	panelBody := a.styles.panelBody.Width(max(10, width-4)).Height(bodyHeight).Render(body)
	return style.Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, header, panelBody))
}

func (a App) renderMessageBlock(message provider.Message, width int) string {
	switch message.Role {
	case "event":
		return a.styles.inlineNotice.Width(width).Render("  > " + wrapPlain(message.Content, max(16, width-6)))
	case "error":
		return a.styles.inlineError.Width(width).Render("  ! " + wrapPlain(message.Content, max(16, width-6)))
	case "system":
		return a.styles.inlineSystem.Width(width).Render("  - " + wrapPlain(message.Content, max(16, width-6)))
	}
	maxMessageWidth := clamp(width, 24, max(24, int(float64(width)*0.92)))
	tag := "[ NEO ]"
	tagStyle := a.styles.messageAgentTag
	bodyStyle := a.styles.messageBody
	switch message.Role {
	case "user":
		tag = "[ YOU ]"
		tagStyle = a.styles.messageUserTag
		bodyStyle = a.styles.messageUserBody
	case "tool":
		tag = "[ TOOL ]"
		tagStyle = a.styles.messageToolTag
		bodyStyle = a.styles.messageToolBody
	}
	content := strings.TrimSpace(message.Content)
	if content == "" && len(message.ToolCalls) > 0 {
		names := make([]string, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			names = append(names, call.Name)
		}
		content = "Tool calls: " + strings.Join(names, ", ")
	}
	if content == "" {
		content = "(empty)"
	}
	return lipgloss.JoinVertical(lipgloss.Left, tagStyle.Render(tag), a.renderMessageContent(content, maxMessageWidth-2, bodyStyle))
}

func (a App) renderMessageContent(content string, width int, bodyStyle lipgloss.Style) string {
	parts := strings.Split(content, "```")
	if len(parts) == 1 {
		return bodyStyle.Width(width).Render(wrapPlain(content, max(16, width-2)))
	}
	blocks := make([]string, 0, len(parts))
	for i, part := range parts {
		if i%2 == 0 {
			trimmed := strings.Trim(part, "\n")
			if trimmed == "" {
				continue
			}
			blocks = append(blocks, bodyStyle.Width(width).Render(wrapPlain(trimmed, max(16, width-2))))
			continue
		}
		code := strings.Trim(part, "\n")
		lines := strings.Split(code, "\n")
		if len(lines) > 1 && !strings.Contains(lines[0], " ") && !strings.Contains(lines[0], "\t") {
			code = strings.Join(lines[1:], "\n")
		}
		blocks = append(blocks, a.styles.codeBlock.Width(width).Render(a.styles.codeText.Width(max(10, width-4)).Render(code)))
	}
	if len(blocks) == 0 {
		return bodyStyle.Width(width).Render("(empty)")
	}
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}

func (a App) statusBadge(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "failed"):
		return a.styles.badgeError.Render(text)
	case a.state.IsAgentRunning || strings.Contains(lower, "running") || strings.Contains(lower, "thinking"):
		return a.styles.badgeWarning.Render(text)
	default:
		return a.styles.badgeSuccess.Render(text)
	}
}

func (a App) focusLabel() string {
	switch a.focus {
	case panelSessions:
		return "Sessions"
	case panelTranscript:
		return "Transcript"
	default:
		return "Composer"
	}
}

func (a App) computeLayout() layout {
	contentWidth := max(80, a.width-4)
	helpHeight := 2
	if a.state.ShowHelp {
		helpHeight = 6
	}
	contentHeight := max(18, a.height-7-helpHeight)
	lay := layout{contentWidth: contentWidth, contentHeight: contentHeight}
	if contentWidth < 110 {
		lay.stacked = true
		lay.sidebarWidth = contentWidth
		lay.sidebarHeight = clamp(contentHeight/3, 9, 13)
		lay.rightWidth = contentWidth
		lay.rightHeight = max(10, contentHeight-lay.sidebarHeight)
		return lay
	}
	lay.sidebarWidth = 30
	lay.sidebarHeight = contentHeight
	lay.rightWidth = contentWidth - lay.sidebarWidth
	lay.rightHeight = contentHeight
	return lay
}

func (a App) isFilteringSessions() bool {
	return a.sessions.FilterState() != list.Unfiltered
}

func ListenForRuntimeEvent(sub <-chan agentruntime.RuntimeEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-sub
		if !ok {
			return RuntimeClosedMsg{}
		}
		return RuntimeMsg{Event: event}
	}
}

func runAgent(runtime agentruntime.Runtime, sessionID string, content string) tea.Cmd {
	return func() tea.Msg {
		err := runtime.Run(context.Background(), agentruntime.UserInput{SessionID: sessionID, Content: content})
		return runFinishedMsg{err: err}
	}
}

func runLocalCommand(configManager *config.Manager, raw string) tea.Cmd {
	return func() tea.Msg {
		notice, err := executeLocalCommand(context.Background(), configManager, raw)
		return localCommandResultMsg{notice: notice, err: err}
	}
}

func runModelSelection(configManager *config.Manager, model string) tea.Cmd {
	return func() tea.Msg {
		notice, err := setCurrentModel(context.Background(), configManager, model)
		return localCommandResultMsg{notice: notice, err: err}
	}
}

func executeLocalCommand(ctx context.Context, configManager *config.Manager, raw string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty command")
	}
	if !strings.EqualFold(fields[0], "/set") {
		return "", fmt.Errorf("unknown command %q", fields[0])
	}
	if len(fields) < 3 {
		return "", fmt.Errorf("usage: /set url <base_url> | /set key <api_key> | /set model <model_name>")
	}
	value := strings.TrimSpace(strings.Join(fields[2:], " "))
	if value == "" {
		return "", fmt.Errorf("command value is empty")
	}
	switch strings.ToLower(fields[1]) {
	case "url":
		if _, err := url.ParseRequestURI(value); err != nil {
			return "", fmt.Errorf("invalid url: %w", err)
		}
		if err := configManager.Update(ctx, func(cfg *config.Config) error {
			selectedName := strings.TrimSpace(cfg.SelectedProvider)
			for i := range cfg.Providers {
				if strings.EqualFold(strings.TrimSpace(cfg.Providers[i].Name), selectedName) {
					cfg.Providers[i].BaseURL = value
					return nil
				}
			}
			return fmt.Errorf("selected provider %q not found", cfg.SelectedProvider)
		}); err != nil {
			return "", err
		}
		cfg := configManager.Get()
		return fmt.Sprintf("[System] Base URL updated for %s -> %s", cfg.SelectedProvider, value), nil
	case "key":
		cfg := configManager.Get()
		selected, err := cfg.SelectedProviderConfig()
		if err != nil {
			return "", err
		}
		if err := configManager.UpsertEnv(selected.APIKeyEnv, value); err != nil {
			return "", fmt.Errorf("persist api key: %w", err)
		}
		if err := configManager.OverloadManagedEnvironment(); err != nil {
			return "", fmt.Errorf("reload managed env: %w", err)
		}
		if _, err := configManager.Reload(ctx); err != nil {
			return "", fmt.Errorf("reload config: %w", err)
		}
		return fmt.Sprintf("[System] %s updated and loaded.", selected.APIKeyEnv), nil
	case "model":
		return setCurrentModel(ctx, configManager, value)
	default:
		return "", fmt.Errorf("unsupported /set field %q", fields[1])
	}
}

func setCurrentModel(ctx context.Context, configManager *config.Manager, model string) (string, error) {
	if err := configManager.Update(ctx, func(cfg *config.Config) error {
		cfg.CurrentModel = model
		selectedName := strings.TrimSpace(cfg.SelectedProvider)
		for i := range cfg.Providers {
			if strings.EqualFold(strings.TrimSpace(cfg.Providers[i].Name), selectedName) {
				cfg.Providers[i].Model = model
				return nil
			}
		}
		return nil
	}); err != nil {
		return "", err
	}
	if _, err := configManager.Reload(ctx); err != nil {
		return "", fmt.Errorf("reload config: %w", err)
	}
	return fmt.Sprintf("[System] Current model switched to %s.", model), nil
}
