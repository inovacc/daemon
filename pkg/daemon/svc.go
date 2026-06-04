package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kardianos/service"
)

// osService is the subset of kardianos service.Service that the svc verbs use.
// Defining it as an interface lets unit tests inject a fake; the real
// service.Service satisfies it. Constructed via the newOSService seam.
type osService interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
	Status() (service.Status, error)
	Run() error
}

// newOSService is the seam used to build the OS service handle. Tests override it
// to inject a fake; production uses realOSService (kardianos-backed).
var newOSService = realOSService

// program is the kardianos service body. It wraps the daemon supervisor: the OS
// service manager calls Start when the service starts and Stop on shutdown.
type program struct {
	o           Options
	run         func(ctx context.Context, o Options) error // supervisor seam (defaults to RunMonitor)
	stopTimeout time.Duration

	cancel context.CancelFunc
	done   chan struct{}
}

// newProgram builds a program bound to o. o is expected to be withDefaults()'d.
func newProgram(o Options) *program {
	return &program{
		o:           o,
		run:         RunMonitor,
		stopTimeout: 10 * time.Second,
	}
}

// Start launches the supervisor in a cancelable goroutine and returns immediately,
// as required by the kardianos service.Interface contract.
func (p *program) Start(service.Service) error {
	log := p.o.logger().With(slog.String("role", "os-service"))
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := p.run(ctx, p.o); err != nil {
			log.Error("supervisor exited with error", slog.Any("err", err))
		}
	}()
	log.Info("os service started")
	return nil
}

// Stop cancels the supervisor context and waits for it to drain, up to stopTimeout,
// then returns so the OS service manager can terminate the process.
func (p *program) Stop(service.Service) error {
	log := p.o.logger().With(slog.String("role", "os-service"))
	if p.cancel != nil {
		p.cancel()
	}
	if p.done == nil {
		return nil
	}
	select {
	case <-p.done:
		log.Info("os service stopped")
	case <-time.After(p.stopTimeout):
		log.Warn("os service stop timed out; forcing exit", slog.Duration("timeout", p.stopTimeout))
	}
	return nil
}

// realOSService constructs a kardianos service.Service wrapping a program for o.
// It guards the empty ServiceName case with a friendly error before service.New
// (which would otherwise return the opaque service.ErrNameFieldRequired).
func realOSService(o Options) (osService, error) {
	if o.ServiceName == "" {
		return nil, fmt.Errorf("daemon: cannot manage OS service: ServiceName is empty (set Options.BinaryName or Options.ServiceName)")
	}
	cfg := &service.Config{
		Name:        o.ServiceName,
		DisplayName: o.ServiceName,
		Description: fmt.Sprintf("%s service", o.BinaryName),
		Arguments:   []string{"svc", "run"},
	}
	s, err := service.New(newProgram(o), cfg)
	if err != nil {
		return nil, fmt.Errorf("daemon: build OS service: %w", err)
	}
	return s, nil
}
