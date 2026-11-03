//go:build !windows

package identity

func dpapiAvailable() bool               { return false }
func protect(_ []byte) ([]byte, error)   { panic("DPAPI unavailable") }
func unprotect(_ []byte) ([]byte, error) { panic("DPAPI unavailable") }
