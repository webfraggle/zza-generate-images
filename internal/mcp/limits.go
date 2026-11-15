package mcp

// Limits of the MCP tools (see the "Limits" table in the design spec).
const (
	maxTextFileBytes    = 256 << 10 // write_file content
	maxFilesPerTemplate = 64        // checked only when creating new files
	maxTemplateBytes    = 30 << 20  // template folder size after a write

	maxUploadBytes       = 5 << 20 // upload_image, decoded
	maxUploadBase64Chars = 7 << 20 // upload_image, base64 string
	maxUploadSide        = 4096

	maxImportBytes      = 20 << 20 // import_image source file
	maxImportSide       = 8192
	maxStoredImportSide = 2048

	maxToolInputBytes  = 64 << 20   // photo tools input file
	maxToolInputPixels = 25_000_000 // photo tools input
	maxToolInputSide   = 8192
	maxToolOutputSide  = 2048

	maxSVGBytes = 256 << 10
	maxSVGSide  = 1024
	maxSVGPaths = 2000

	maxPreviewDataBytes = 64 << 10 // render_preview data
	maxSheetFrames      = 12       // GIF contact sheet
	sheetColumns        = 4
	maxSheetSide        = 2048

	maxResultImageSide = 768 // images returned for display only

	maxSamplePoints = 64
	maxSampleRadius = 5

	maxFontBytes   = 50 << 20
	maxFontResults = 200

	canvasSize     = 240
	templateSuffix = "-105-v1"
)
