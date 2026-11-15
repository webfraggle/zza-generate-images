// Package docs embeds documentation that the desktop MCP server hands to AI clients.
package docs

import _ "embed"

//go:embed yaml-template-spec.md
var TemplateSpec string

//go:embed ai-design-guide.md
var DesignGuide string
