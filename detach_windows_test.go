//go:build windows

package daemon

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestDetachedSysProcAttr(t *testing.T) {
	attr := detachedSysProcAttr()
	if attr == nil {
		t.Fatal("detachedSysProcAttr returned nil")
	}

	if !attr.HideWindow {
		t.Error("HideWindow must be true for a detached child")
	}

	want := uint32(windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	if attr.CreationFlags != want {
		t.Errorf("CreationFlags = %#x, want %#x", attr.CreationFlags, want)
	}
}
