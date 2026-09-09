//go:build windows

package pathutil

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var getFinalPathNameByHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")

// Resolve resolves an existing file or directory without listing its ancestors.
func Resolve(path string) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	// Resolve junctions through the accessible directory itself. EvalSymlinks
	// normalizes each ancestor with FindFirstFile, which requires listing rights
	// outside the readable root of a restricted Windows sandbox.
	const fileReadAttributes = 0x0080
	handle, err := syscall.CreateFile(name, fileReadAttributes,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "resolve path", Path: path, Err: err}
	}
	defer syscall.CloseHandle(handle)
	buffer := make([]uint16, 1024)
	for {
		length, _, callErr := getFinalPathNameByHandle.Call(uintptr(handle), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
		if length == 0 {
			return "", &os.PathError{Op: "resolve path", Path: path, Err: callErr}
		}
		if length >= uintptr(len(buffer)) {
			buffer = make([]uint16, length+1)
			continue
		}
		resolved := syscall.UTF16ToString(buffer[:length])
		if strings.HasPrefix(resolved, `\\?\UNC\`) {
			return `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`), nil
		}
		return strings.TrimPrefix(resolved, `\\?\`), nil
	}
}
