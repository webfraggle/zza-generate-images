package mcp

import (
	"io"
	"os"
)

// readRegularFile opens path once (without following a final symlink and
// without blocking on FIFOs where the platform supports it), requires a
// regular file and reads at most limit bytes. It returns errNotRegular or
// errFileTooLarge for those cases.
//
// Platform-independent hardening (Windows has no O_NOFOLLOW/O_NONBLOCK):
// the path is Lstat'ed first (symlinks and non-regular files are refused
// before opening), and the opened file must be the same file (os.SameFile)
// as the one Lstat saw, so a swap between the two calls is detected.
func readRegularFile(path string, limit int64) ([]byte, error) {
	lfi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !lfi.Mode().IsRegular() {
		return nil, errNotRegular
	}
	f, err := os.OpenFile(path, os.O_RDONLY|openFlags, 0)
	if err != nil {
		if isSymlinkLoop(err) {
			return nil, errNotRegular
		}
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || !os.SameFile(lfi, fi) {
		return nil, errNotRegular
	}
	if fi.Size() > limit {
		return nil, errFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errFileTooLarge
	}
	return data, nil
}
