package state

type Mode int

const (
	ModeChat Mode = iota
	ModeHelp
)

type FocusTarget int

const (
	FocusInput FocusTarget = iota
	FocusMain
	FocusSide
)

type UIState struct {
	Width           int
	Height          int
	Mode            Mode
	Focus           FocusTarget
	AutoScroll      bool
	ApprovalRunning bool
	SystemExpanded  bool
	SideCollapsed   bool
	SideNarrowOpen  bool
	StatusMessage   string
	LastError       string
	CommandHistory  []string
	CmdHistIndex    int
	CommandDraft    string
}
