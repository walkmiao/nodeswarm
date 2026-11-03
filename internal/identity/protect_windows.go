//go:build windows

package identity

import (
	"errors"
	"syscall"
	"unsafe"
)

const cryptProtectUIForbidden = 0x1

type dataBlob struct {
	size uint32
	data *byte
}

var (
	crypt32       = syscall.NewLazyDLL("crypt32.dll")
	protectData   = crypt32.NewProc("CryptProtectData")
	unprotectData = crypt32.NewProc("CryptUnprotectData")
	localFree     = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")
)

func dpapiAvailable() bool { return true }

func protect(raw []byte) ([]byte, error)   { return transform(raw, true) }
func unprotect(raw []byte) ([]byte, error) { return transform(raw, false) }

func transform(raw []byte, encrypt bool) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty device token")
	}
	input := dataBlob{size: uint32(len(raw)), data: &raw[0]}
	var output dataBlob
	// Optional entropy keeps this token category separate from unrelated
	// secrets under the same Windows user account.
	entropy := []byte("NodeSwarm-device-token-v1")
	ent := dataBlob{size: uint32(len(entropy)), data: &entropy[0]}
	fn := protectData
	if !encrypt {
		fn = unprotectData
	}
	r, _, err := fn.Call(
		uintptr(unsafe.Pointer(&input)),
		0, // description; nil on encrypt, output description ignored on decrypt
		uintptr(unsafe.Pointer(&ent)),
		0, 0, cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&output)),
	)
	if r == 0 {
		if err == syscall.Errno(0) {
			err = errors.New("DPAPI failed")
		}
		return nil, err
	}
	defer localFree.Call(uintptr(unsafe.Pointer(output.data)))
	if output.data == nil || output.size == 0 {
		return nil, errors.New("empty DPAPI result")
	}
	out := make([]byte, int(output.size))
	copy(out, unsafe.Slice(output.data, int(output.size)))
	return out, nil
}
