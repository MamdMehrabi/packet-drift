package xray

// State represents the lifecycle state of the Xray process.
type State int

const (
	// StateStopped means no Xray process is running.
	StateStopped State = iota
	// StateRunning means an Xray process is active.
	StateRunning
)

func (s State) String() string {
	switch s {
	case StateRunning:
		return "running"
	default:
		return "stopped"
	}
}
