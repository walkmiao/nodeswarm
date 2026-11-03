//go:build !windows

package main

func acquireAppLock() (func(), error) { return func() {}, nil }
