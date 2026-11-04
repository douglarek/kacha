//go:build !darwin

// Package macwin drives the windows of macOS from code mygo does not cover.
package macwin

import "github.com/egoist/mygo"

// CoverMenuBar does nothing outside macOS.
func CoverMenuBar(*mygo.Window) {}

// Activate does nothing outside macOS.
func Activate(*mygo.Window) {}

// AllowKeys does nothing outside macOS.
func AllowKeys() {}
