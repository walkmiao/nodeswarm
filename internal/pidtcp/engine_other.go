//go:build !windows || !amd64

package pidtcp

import (
	"context"
	"errors"
)

func runWindows(ctx context.Context, routes []Route, dir string, ready chan<- error) error {
	err := errors.New("experimental PID TCP engine requires Windows amd64 and WinDivert 2.2 driver")
	ready <- err
	return err
}
