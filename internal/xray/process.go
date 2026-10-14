package xray

import (
	"errors"
	"os"
	"os/exec"
	"sync"
)

// Process manages a single Xray OS process. Only one instance may be active
// at a time; concurrent calls are synchronized by the mutex.
type Process struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{} // closed when the tracked process has fully exited
}

// NewProcess creates an empty process manager.
func NewProcess() *Process {
	return &Process{}
}

// Start launches the Xray binary with the given config file.
// It returns ErrAlreadyRunning if a process is already active,
// ErrInvalidExecutable if the binary does not exist, and
// ErrInvalidConfig if the config file does not exist.
func (p *Process) Start(binaryPath, configPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		return ErrAlreadyRunning
	}

	if _, err := os.Stat(binaryPath); err != nil {
		return errors.Join(ErrInvalidExecutable, err)
	}
	if _, err := os.Stat(configPath); err != nil {
		return errors.Join(ErrInvalidConfig, err)
	}

	cmd := exec.Command(binaryPath, "run", "-c", configPath)
	if err := cmd.Start(); err != nil {
		return errors.Join(ErrStartFailed, err)
	}

	done := make(chan struct{})
	p.cmd = cmd
	p.done = done

	// Reap the process and clear tracking as soon as it exits,
	// whether killed explicitly or on its own. Wait may only be
	// called once per Cmd, so this goroutine is its sole caller.
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		if p.cmd == cmd {
			p.cmd = nil
			p.done = nil
		}
		p.mu.Unlock()
		close(done)
	}()

	return nil
}

// Stop terminates the tracked process and waits for it to exit.
// It returns ErrNotRunning if no process is active.
func (p *Process) Stop() error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	if cmd == nil {
		p.mu.Unlock()
		return ErrNotRunning
	}
	p.cmd = nil
	p.done = nil
	p.mu.Unlock()

	// Best-effort kill: the process may have exited on its own in the
	// meantime. The wait goroutine reaps it and closes done either way.
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-done
	return nil
}

// IsRunning reports whether a process is currently tracked and alive.
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}
