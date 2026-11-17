//go:build darwin || linux

package record

import (
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// tempState points the state of a film at a folder of the test's own.
func tempState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ClearState()
	t.Cleanup(ClearState)
}

func TestFilmingStops(t *testing.T) {
	tempState(t)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(reaped)
	}()
	pid := cmd.Process.Pid
	if err := write(pidFile(), strconv.Itoa(pid)); err != nil {
		t.Fatal(err)
	}
	if err := write(pathFile(), "/tmp/film.mp4"); err != nil {
		t.Fatal(err)
	}

	if got, ok := Filming(); !ok || got != pid {
		t.Fatalf("filming = %d, %v, want the process %d", got, ok, pid)
	}
	path, err := StopFilming(pid)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-reaped:
	case <-time.After(5 * time.Second):
		t.Error("the recorder went on after the stop")
	}
	if path != "/tmp/film.mp4" {
		t.Errorf("path = %q, want the file the film left", path)
	}
	if _, ok := Filming(); ok {
		t.Error("a film that ended is still marked")
	}
}

func TestFilmingForgetsWhatEnded(t *testing.T) {
	tempState(t)
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := write(pidFile(), strconv.Itoa(cmd.Process.Pid)); err != nil {
		t.Fatal(err)
	}
	if pid, ok := Filming(); ok {
		t.Errorf("filming = %d, want no film", pid)
	}
	if _, err := readPID(); err == nil {
		t.Error("the mark of a film that ended stayed")
	}
}

func TestSettledLeavesTheFile(t *testing.T) {
	tempState(t)
	if err := write(pidFile(), "1"); err != nil {
		t.Fatal(err)
	}
	Settled("/tmp/film.mp4")
	if got := readPath(); got != "/tmp/film.mp4" {
		t.Errorf("path = %q, want it left for the key that stops", got)
	}
	if _, err := readPID(); err == nil {
		t.Error("the mark of a film stayed after it settled")
	}
}
