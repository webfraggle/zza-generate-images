package desktop

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// RunGUI opens a Wails-powered native window hosting handler. When Wails
// cannot start (no WebView2 on Windows 10, exotic Linux, headless CI),
// it falls back to opening the system default browser pointed at a localhost
// HTTP server that serves the same handler. Blocks until the window closes
// or the server stops.
func RunGUI(title string, handler http.Handler) error {
	err := wails.Run(&options.App{
		Title:  title,
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
	})
	if err == nil {
		return nil
	}
	log.Printf("wails unavailable (%v) — falling back to default browser", err)
	return RunBrowser(handler)
}

// RunBrowser starts an HTTP server on 127.0.0.1:0 and opens the default browser.
// Blocks until the server stops (typically via Ctrl+C in the terminal).
func RunBrowser(handler http.Handler) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("desktop: listen: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	log.Printf("zza editor running at %s (close terminal to quit)", url)

	go openInBrowser(url)
	return http.Serve(listener, handler)
}

// RunServerOnly starts an HTTP server on the given address without opening
// anything. Used by `zza serve`.
func RunServerOnly(addr string, handler http.Handler) error {
	log.Printf("zza serving on %s", addr)
	return http.ListenAndServe(addr, handler)
}

func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("desktop: open browser: %v", err)
	}
}
