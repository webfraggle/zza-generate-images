package version

// Version is set at build time via -ldflags.
// Falls back to "dev" when built without ldflags (e.g. go run).
var Version = "dev"
