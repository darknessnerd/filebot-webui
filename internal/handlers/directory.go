package handlers

import (
	"github.com/gin-gonic/gin"
	"os"
	"path/filepath"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
)

// DirectoryHandler handles directory browsing and selection
type DirectoryHandler struct {
	config      *config.Config
	db          *database.DB
	authService *auth.Service
}

// NewDirectoryHandler creates a new directory handler
func NewDirectoryHandler(config *config.Config, db *database.DB, authService *auth.Service) *DirectoryHandler {
	return &DirectoryHandler{
		config:      config,
		db:          db,
		authService: authService,
	}
}

// BrowseDirectory handles directory browsing and selection
func (h *DirectoryHandler) BrowseDirectory(c *gin.Context) {
	// Directory presets from config
	presets := h.config.DirectoryPresets
	presetNames := []string{}
	presetRoots := map[string]string{} // Add this to pass to template
	for name, root := range presets {
		presetNames = append(presetNames, name)
		presetRoots[name] = root
	}

	// Get selected preset from query, default to first
	selectedPreset := c.DefaultQuery("preset", "")
	rootPath := ""
	if selectedPreset != "" && presets[selectedPreset] != "" {
		rootPath = presets[selectedPreset]
	} else if len(presetNames) > 0 {
		selectedPreset = presetNames[0]
		rootPath = presets[selectedPreset]
	} else {
		rootPath = "."
	}

	currentPath := c.Query("path")
	if currentPath == "" {
		// Initial load: show files of rootPath
		currentPath = rootPath
	}
	if currentPath != rootPath {
		// If preset changed, reset currentPath to rootPath
		presetParam := c.Query("preset")
		if presetParam != "" && presetParam != selectedPreset {
			currentPath = rootPath
		}
	}
	parentPath := filepath.Dir(currentPath)
	if parentPath == "." || parentPath == "/" {
		parentPath = rootPath
	}

	// Collapse state (simple query param for demo)
	isCollapsed := c.DefaultQuery("collapsed", "false") == "true"

	// Selection state (simple query param for demo)
	selectedItems := map[string]bool{}
	if sel := c.QueryArray("selected"); len(sel) > 0 {
		for _, s := range sel {
			selectedItems[s] = true
		}
	}

	// List files and directories
	entries, err := os.ReadDir(currentPath)
	files := []map[string]string{}
	fileCount := 0
	dirCount := 0
	for _, entry := range entries {
		itemType := "file"
		if entry.IsDir() {
			itemType = "directory"
			dirCount++
		} else {
			fileCount++
		}
		files = append(files, map[string]string{
			"Name": entry.Name(),
			"Type": itemType,
			"Path": filepath.Join(currentPath, entry.Name()),
		})
	}

	// Selection mode (for demo, can be from query)
	selectionMode := c.DefaultQuery("selectionMode", "both")

	// Is current directory selected
	isSelectedDirectory := selectedItems[filepath.Base(currentPath)]

	// Helper for template
	isCheckboxAvailable := func(item map[string]string) bool {
		return selectionMode == "both" ||
			(selectionMode == "files" && item["Type"] == "file") ||
			(selectionMode == "directories" && item["Type"] == "directory")
	}
	isSelected := func(item map[string]string) bool {
		return selectedItems[item["Name"]]
	}

	// Get action from query param (default to "files")
	action := c.DefaultQuery("action", "none")

	// Check for embedded parameter (for modal view)
	isEmbedded := c.Query("embedded") == "true"
	torrentID := c.Query("torrent_id")
	filesJSON := c.Query("files")

	// Choose template based on embedded mode
	templateName := "directory_browser.html"
	if isEmbedded {
		templateName = "directory_browser_embedded.html"
	}

	// Render template
	RenderWithHTMX(c, templateName, gin.H{
		"CurrentPath":          currentPath,
		"RootPath":             rootPath,
		"ParentPath":           parentPath,
		"AvailableDirectories": []string{rootPath},
		"Files":                files,
		"FileCount":            fileCount,
		"DirectoryCount":       dirCount,
		"SelectionMode":        selectionMode,
		"IsCollapsed":          isCollapsed,
		"IsSelectedDirectory":  isSelectedDirectory,
		"IsCheckboxAvailable":  isCheckboxAvailable,
		"IsSelected":           isSelected,
		"Loading":              false,
		"Error":                err,
		"Action":               action,
		"DirectoryPresets":     presets,
		"PresetNames":          presetNames,
		"PresetRoots":          presetRoots,
		"SelectedPreset":       selectedPreset,
		"TorrentID":            torrentID,
		"FilesJSON":            filesJSON,
		"IsEmbedded":           isEmbedded,
	}, true)
}
