package mcp

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// copyTemplates copies the named template folders from the repo's templates/
// into <t.TempDir()>/templates and returns that absolute path. Without names
// the returned folder is empty.
func copyTemplates(t *testing.T, names ...string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		src := filepath.Join("..", "..", "templates", n)
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatalf("copyTemplates: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(dst, n), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			in, err := os.Open(filepath.Join(src, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			out, err := os.Create(filepath.Join(dst, n, e.Name()))
			if err != nil {
				in.Close()
				t.Fatal(err)
			}
			_, err = io.Copy(out, in)
			in.Close()
			out.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	abs, err := filepath.Abs(dst)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// newSession starts an in-memory MCP server on templatesDir and returns a
// connected client session. Both ends are closed on test cleanup.
func newSession(t *testing.T, templatesDir string) *sdk.ClientSession {
	t.Helper()
	srv, err := NewServer(Config{TemplatesDir: templatesDir})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx := context.Background()
	st, ct := sdk.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Close()
	})
	return cs
}

// callTool calls a tool and fails the test on protocol-level errors.
// Tool-level errors are returned as results with IsError set.
func callTool(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

// resultText joins all text contents of a result.
func resultText(res *sdk.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// resultImage decodes the first image content of a result.
func resultImage(t *testing.T, res *sdk.CallToolResult) image.Image {
	t.Helper()
	for _, c := range res.Content {
		if ic, ok := c.(*sdk.ImageContent); ok {
			img, _, err := image.Decode(bytes.NewReader(ic.Data))
			if err != nil {
				t.Fatalf("decoding result image: %v", err)
			}
			return img
		}
	}
	t.Fatal("result has no image content")
	return nil
}

func requireOK(t *testing.T, res *sdk.CallToolResult) {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(res))
	}
}

func requireToolError(t *testing.T, res *sdk.CallToolResult, substr string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("expected tool error containing %q, got success: %s", substr, resultText(res))
	}
	if !strings.Contains(resultText(res), substr) {
		t.Fatalf("tool error %q does not contain %q", resultText(res), substr)
	}
}
