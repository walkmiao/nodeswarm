//go:build windows

package main

import "testing"

func TestOnlyOneDesktopInstance(t *testing.T) {
	release, err := acquireAppLock()
	if err != nil {
		t.Fatal(err)
	}
	if second, e := acquireAppLock(); e == nil {
		second()
		t.Fatal("duplicate desktop instance permitted")
	}
	release()
	next, err := acquireAppLock()
	if err != nil {
		t.Fatal("mutex not released: ", err)
	}
	next()
}
