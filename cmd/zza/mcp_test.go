package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMCPCmd_StdoutOnlyProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "zza")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// Templates dir with one template copied from the repo.
	tdir := filepath.Join(tmp, "templates")
	src := filepath.Join("..", "..", "templates", "sbb-105-v1")
	dst := filepath.Join(tdir, "sbb-105-v1")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(bin, "mcp", "--templates-dir", tdir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	if _, err := io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`+"\n"); err != nil {
		t.Fatal(err)
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	select {
	case line := <-lines:
		var msg struct {
			ID     float64 `json:"id"`
			Result struct {
				ServerInfo struct {
					Name string `json:"name"`
				} `json:"serverInfo"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("first stdout line is not JSON: %q", line)
		}
		if msg.ID != 1 || msg.Result.ServerInfo.Name != "zza" {
			t.Fatalf("unexpected response: %s", line)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("no response within 20s")
	}

	_ = stdin.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit after stdin closed")
	}
	for line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			t.Errorf("non-JSON line on stdout: %q", line)
		}
	}
	if !strings.Contains(stderr.String(), "zza mcp: templates=") {
		t.Errorf("stderr lacks startup log: %q", stderr.String())
	}
}
