package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	funcs  []func()
	done   chan struct{}
	mu     sync.Mutex
}

func New() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	m := &Manager{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	m.funcs = append(m.funcs, stop)
	m.funcs = append(m.funcs, func() { close(m.done) })

	go func() {
		<-ctx.Done()
		m.mu.Lock()
		funcs := m.funcs
		m.mu.Unlock()

		slog.Info("shutdown started", "process_total", len(funcs))
		for _, f := range funcs {
			f()
		}
		slog.Info("shutdown finished")
	}()

	return m
}

func (m *Manager) Context() context.Context { return m.ctx }
func (m *Manager) Cancel()                  { m.cancel() }
func (m *Manager) Add(f func())             { m.funcs = append(m.funcs, f) }
func (m *Manager) Done() <-chan struct{}    { return m.done }
