//go:build !windows

package main

import "testing"

func TestRunAsServiceIsNoopOnNonWindows(t *testing.T) {
	if runAsService() {
		t.Fatal("runAsService() should be false on non-Windows platforms")
	}
}
