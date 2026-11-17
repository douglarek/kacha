//go:build darwin || linux

package record

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// stopWait is how long a film may take to write its file, as the key that
// stops one asks it to.
const stopWait = 20 * time.Second

// Filming returns the process that films, if one is, and forgets a film that
// ended without a word from it.
func Filming() (int, bool) {
	pid, err := readPID()
	if err != nil || pid <= 0 || !alive(pid) {
		ClearState()
		return 0, false
	}
	return pid, true
}

// StopFilming ends the film the process pid makes, waits for its file, and
// returns where it went.
func StopFilming(pid int) (string, error) {
	if !alive(pid) {
		ClearState()
		return "", errors.New("kacha: no film is being made")
	}
	// SIGUSR1, which the film handles as a stop: SIGINT would quit the app
	// of the film before it says where the film went.
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		Logf("record: stop: signal %d: %v", pid, err)
		return "", fmt.Errorf("kacha: stop recording: %w", err)
	}
	Logf("record: stop: asked %d to end its film", pid)
	for deadline := time.Now().Add(stopWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if !alive(pid) {
			break
		}
	}
	if alive(pid) {
		// The film is still there, and the key that stops it has not given
		// up on it: a press again asks once more.
		Logf("record: stop: %d went on filming", pid)
		return "", fmt.Errorf("kacha: the film of %d did not end", pid)
	}
	path := readPath()
	ClearState()
	return path, nil
}

// Detach starts the app exe with args in a session of its own, so that the key
// that starts a film returns at once, and marks the film it makes.
func Detach(exe string, args []string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("kacha: record: %w", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	ClearState()
	if err := write(pidFile(), strconv.Itoa(pid)); err != nil {
		return pid, err
	}
	return pid, nil
}

// Unmark forgets the film whose process is ending, the file it went to
// aside.
func Unmark() { os.Remove(pidFile()) }

// Settled tells the key that stops a film where its file went, which ends the
// film.
func Settled(path string) {
	if err := write(pathFile(), path); err != nil {
		Logf("record: %v", err)
	}
	if err := os.Remove(pidFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		Logf("record: %v", err)
	}
}

// ClearState forgets the film there was, if any.
func ClearState() {
	os.Remove(pidFile())
	os.Remove(pathFile())
}

// alive reports whether the process pid runs.
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// stateDir returns where a film keeps what ends it, which goes with the
// session.
func stateDir() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			dir = cache
		} else {
			dir = os.TempDir()
		}
	}
	dir = filepath.Join(dir, "kacha")
	os.MkdirAll(dir, 0o755)
	return dir
}

func pidFile() string  { return filepath.Join(stateDir(), "record.pid") }
func pathFile() string { return filepath.Join(stateDir(), "record.path") }

// readPID returns the process marked as filming.
func readPID() (int, error) {
	data, err := os.ReadFile(pidFile())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// readPath returns the file the last film went to.
func readPath() string {
	data, err := os.ReadFile(pathFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// write puts data at path.
func write(path, data string) error {
	return os.WriteFile(path, []byte(data+"\n"), 0o644)
}
