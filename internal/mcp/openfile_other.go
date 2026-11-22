//go:build !unix

package mcp

const openFlags = 0

func isSymlinkLoop(error) bool { return false }
