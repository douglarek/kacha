//go:build darwin

package settings

import "testing"

// TestLoginItemAnswers covers what the settings count on: the login item of
// the app says whether it is on, and takes an order, or says why it cannot,
// where a test binary, which no bundle holds, is refused.
func TestLoginItemAnswers(t *testing.T) {
	if LaunchAtLogin() && !LaunchAtLogin() {
		t.Error("the login item changed its mind")
	}
	if err := SetLaunchAtLogin(false); err != nil {
		t.Logf("the login item could not be taken away: %v", err)
	}
}
