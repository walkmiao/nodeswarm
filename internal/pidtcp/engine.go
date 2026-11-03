package pidtcp

import (
	"context"
	"errors"
	"sync"
)

// Engine is an explicitly opted-in experimental TCP-only redirector.
// Readiness means the driver opened, not that a particular game's network
// traffic has been verified or that a system-wide fail-closed rule exists.
type Engine struct {
	mu       sync.Mutex
	starting bool
	running  bool
	cancel   context.CancelFunc
	done     chan struct{}
	runErr   error
}

func (e *Engine) Start(routes []Route, driverDir string) error {
	e.mu.Lock()
	if e.running || e.starting {
		e.mu.Unlock()
		return errors.New("PID TCP engine already running")
	}
	if len(routes) == 0 {
		e.mu.Unlock()
		return errors.New("no PID bindings for TCP interception")
	}
	e.starting = true
	e.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		err := runWindows(ctx, routes, driverDir, ready)
		e.mu.Lock()
		e.runErr = err
		e.running = false
		close(done)
		e.mu.Unlock()
	}()
	err := <-ready
	if err != nil {
		cancel()
		<-done
		e.mu.Lock()
		e.starting = false
		e.mu.Unlock()
		return err
	}
	e.mu.Lock()
	e.starting = false
	e.cancel = cancel
	e.done = done
	// The driver can fail between publishing readiness and this lock.
	// Never overwrite its failure with a false "running" state.
	select {
	case <-done:
		err = e.runErr
		if err == nil {
			err = errors.New("PID TCP engine stopped during startup")
		}
		e.running = false
	default:
		e.running = true
		e.runErr = nil
	}
	e.mu.Unlock()
	if err != nil {
		cancel()
		return err
	}
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	cancel, done := e.cancel, e.done
	e.mu.Unlock()
	if cancel == nil || done == nil {
		return
	}
	cancel()
	<-done
	e.mu.Lock()
	if e.done == done {
		e.done = nil
		e.cancel = nil
		e.running = false
	}
	e.mu.Unlock()
}

func (e *Engine) Running() bool { e.mu.Lock(); defer e.mu.Unlock(); return e.running }
func (e *Engine) Err() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runErr
}
