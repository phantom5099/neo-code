package security

type Action string

const (
	ActionDeny  Action = "deny"
	ActionAllow Action = "allow"
	ActionAsk   Action = "ask"
)

type Rule struct {
	Target  string `yaml:"target,omitempty"`
	Command string `yaml:"command,omitempty"`
	Domain  string `yaml:"domain,omitempty"`
	Read    string `yaml:"read,omitempty"`
	Write   string `yaml:"write,omitempty"`
	Exec    string `yaml:"exec,omitempty"`
	Network string `yaml:"network,omitempty"`
}

type Config struct {
	Rules []Rule `yaml:"rules"`
}

type Checker interface {
	Check(toolType string, target string) Action
}
