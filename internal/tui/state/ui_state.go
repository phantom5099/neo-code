package state

type Mode int

const (
	ModeChat Mode = iota
	ModeHelp
)

type UIState struct {
	Width      int
	Height     int
	Mode       Mode
	AutoScroll bool
	CopyStatus string
}
