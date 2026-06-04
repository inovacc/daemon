package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/daemon/pkg/serverinfo"
)

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestMonitorLogsLifecycle(t *testing.T) {
	lg, buf := captureLogger()
	calls := 0
	m := &monitor{
		o:     Options{BinaryName: "t", Logger: lg, GRPCPort: 9501}.withDefaults(),
		guard: newRestartGuard(4, 60*time.Second),
		info:  serverinfo.NewStore(t.TempDir()),
		spawn: func(ctx context.Context, args []string) int {
			calls++
			if calls == 1 {
				return ExitError.AsInt() // crash once...
			}
			return ExitSuccess.AsInt() // ...then clean exit
		},
		sleep: func(time.Duration) {},
	}
	if err := m.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"monitor started", "worker crashed; restarting", "worker exited cleanly", "monitor stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing lifecycle event %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, `"role":"monitor"`) {
		t.Error("missing role=monitor attribute")
	}
}

func TestMonitorLoopAbortLogged(t *testing.T) {
	lg, buf := captureLogger()
	m := &monitor{
		o:     Options{BinaryName: "t", Logger: lg}.withDefaults(),
		guard: newRestartGuard(2, time.Hour),
		info:  serverinfo.NewStore(t.TempDir()),
		spawn: func(ctx context.Context, args []string) int { return ExitError.AsInt() },
		sleep: func(time.Duration) {},
	}
	if err := m.run(context.Background()); err == nil {
		t.Fatal("expected loop-abort error")
	}
	if !strings.Contains(buf.String(), "restart loop detected") {
		t.Errorf("loop-abort not logged:\n%s", buf.String())
	}
}

func TestDefaultLoggerFallback(t *testing.T) {
	o := Options{BinaryName: "t"}.withDefaults()
	if o.logger() == nil {
		t.Error("logger() must never return nil")
	}
}
