package mcp

import (
	"io"
	"os"
)

// readRegularFile opens path once (without following a final symlink and
// without blocking on FIFOs where the platform supports it), requires a
// regular file and reads at most limit bytes. It returns errNotRegular or
// errFileTooLarge for those cases.
func readRegularFile(path string, limit int64) ([]byte, error) {
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
	if !fi.Mode().IsRegular() {
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
