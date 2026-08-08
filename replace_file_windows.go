package main

import (
	"syscall"
	"unsafe"
)

const (
	movefileReplaceExisting = 0x00000001
	movefileWriteThrough    = 0x00000008
)

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFileAtomically(source, destination string) error {
	sourcePath, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPath, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}

	result, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(sourcePath)),
		uintptr(unsafe.Pointer(destinationPath)),
		movefileReplaceExisting|movefileWriteThrough,
	)
	if result == 0 {
		return callErr
	}
	return nil
}
