package handlers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"
)

// Handlers contains all handler instances
type Handlers struct {
	Home        *HomeHandler
	Auth        *AuthHandler
	API         *APIHandler
	Plex        *PlexHandler
	Deluge      *DelugeHandler
	Health      *HealthHandler
	config      *config.Config
	db          *database.DB
	authService *auth.Service
}

// NewHandlers creates a new handlers container with all handler instances
func NewHandlers(config *config.Config, db *database.DB, authService *auth.Service) *Handlers {
	return &Handlers{
		Home:        NewHomeHandler(config, db, authService),
		Plex:        NewPlexHandler(config, db, authService, repository.NewPlexServerRepository(db.DB)),
		Deluge:      NewDelugeHandler(config, db, authService, repository.NewDelugeServerRepository(db.DB)),
		Auth:        NewAuthHandler(config, db, authService),
		API:         NewAPIHandler(config, db, authService),
		Health:      NewHealthHandler(config),
		config:      config,
		db:          db,
		authService: authService,
	}
}

// RenderWithHTMX renders either the content template (for HTMX) or base.html with content embedded
func RenderWithHTMX(c *gin.Context, contentTemplate string, data gin.H, requireAuth bool) {
	logger.Log.Debug().Str("contentTemplate", contentTemplate).Msg("RenderWithHTMX called")
	logger.Log.Debug().Interface("data", data).Msg("RenderWithHTMX: data map values")
	user := (*models.User)(nil)
	data["authenticated"] = false
	if requireAuth {
		userObj, exists := c.Get("user_obj")
		logger.Log.Debug().Bool("exists", exists).Interface("user_obj", userObj).Msg("RenderWithHTMX: user_obj from context")
		if !exists {
			logger.Log.Debug().Msg("RenderWithHTMX: user_obj not found, redirecting to /auth/login")
			c.Redirect(302, "/login")
			return
		}
		ok := false
		user, ok = userObj.(*models.User)
		logger.Log.Debug().Bool("ok", ok).Msg("RenderWithHTMX: user_obj type assertion to *models.User")
		if !ok {
			logger.Log.Debug().Msg("RenderWithHTMX: user_obj invalid type, redirecting to /auth/login")
			c.Redirect(302, "/login")
			return
		}
		var userID int
		var name, email, picture string
		var plex gin.H
		userID = user.ID
		name, email, picture, plex = extractUserInfo(user, user.Name, user.Email)

		userData := gin.H{
			"userID":        userID,
			"name":          name,
			"email":         email,
			"picture":       picture,
			"plex":          plex,
			"authenticated": true,
		}
		for k, v := range userData {
			data[k] = v
		}
	}

	if c.GetHeader("HX-Request") != "" {
		logger.Log.Debug().Str("render_type", "htmx").Str("template", contentTemplate).Msg("RenderWithHTMX: rendering HTMX partial")
		c.HTML(200, contentTemplate, data)
	} else {
		logger.Log.Debug().Str("render_type", "base").Str("template", "base.html").Str("partial", contentTemplate).Msg("RenderWithHTMX: rendering base.html with partial")
		// Render base.html and inject contentTemplate as partial
		// Gin's HTML rendering expects the base template to define a block for content
		// Pass the content template name as "content" key
		data["content"] = contentTemplate
		c.HTML(200, "base.html", data)
	}
}

// Helper to extract user info
func extractUserInfo(user *models.User, nameCtx, emailCtx string) (name, email, picture string, plex gin.H) {
	if user != nil {
		name = user.Name
		email = user.Email
		picture = user.Picture
		plex = gin.H{
			"username": user.PlexUsername,
			"email":    user.PlexEmail,
		}
	} else {
		name = nameCtx
		email = emailCtx
		picture = ""
		plex = nil
	}
	return
}

// DirectoryBrowserHandler handles directory browsing and selection
// Add to Handlers struct if needed
func (h *Handlers) DirectoryBrowser(c *gin.Context) {
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

	// Render template
	RenderWithHTMX(c, "directory_browser.html", gin.H{
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
		"PresetRoots":          presetRoots, // Pass presetRoots to template
		"SelectedPreset":       selectedPreset,
	}, true)
}

// FileBotHandler serves the FileBot form and handles execution
func (h *Handlers) FileBotForm(c *gin.Context) {
	// For initial load, no files or output directory selected
	data := gin.H{
		"FilesJSON":           "",
		"FilesList":           nil,
		"OutputDirectoryJSON": "",
		"OutputDirectory":     "",
		"Status":              "",
		"Errors":              nil,
		"Successes":           nil,
		"Action":              "test", // Set default action to test
	}
	RenderWithHTMX(c, "filebot_form.html", data, true)
}

// FileBotSelection handles updates from the directory browser to the FileBot form
func (h *Handlers) FileBotSelection(c *gin.Context) {
	filesJSON := c.PostForm("files")
	outputDirectoryJSON := c.PostForm("outputDirectory")

	// Preserve selected files even when only output directory is changed
	filesList := []string{}
	if filesJSON != "" {
		if strings.HasPrefix(filesJSON, "[") {
			_ = json.Unmarshal([]byte(filesJSON), &filesList)
		} else {
			filesList = strings.Split(filesJSON, ",")
		}
	}

	outputDirectory := ""
	if outputDirectoryJSON != "" {
		outputDirectory = outputDirectoryJSON
	}

	data := gin.H{
		"FilesJSON":           filesJSON,
		"FilesList":           filesList,
		"OutputDirectoryJSON": outputDirectoryJSON,
		"OutputDirectory":     outputDirectory,
		"Status":              "",
		"Errors":              nil,
		"Successes":           nil,
	}
	RenderWithHTMX(c, "filebot_form.html", data, true)
}

// FileBotExecute runs the FileBot command for selected files and output directory
func (h *Handlers) FileBotExecute(c *gin.Context) {
	// Clear previous status messages
	// This ensures that when execute is clicked, previous messages are cleared

	// Parse form values
	db := c.PostForm("db")
	format := c.PostForm("format")
	action := c.PostForm("action")
	filter := c.PostForm("filter")
	conflictResolution := c.PostForm("conflict_resolution")
	logLevel := c.PostForm("log_level")
	query := c.PostForm("query")
	recursive := c.PostForm("recursive") == "true"
	filesJSON := c.PostForm("files")
	outputDirectoryJSON := c.PostForm("outputDirectory")

	// Multi-file selection logic: parse filesJSON as CSV or JSON array
	var filesList []string
	if filesJSON != "" {
		if strings.HasPrefix(filesJSON, "[") {
			_ = json.Unmarshal([]byte(filesJSON), &filesList)
		} else {
			filesList = strings.Split(filesJSON, ",")
		}
	}
	outputDirectory := outputDirectoryJSON

	successMessages := []string{}
	errorMessages := []string{}
	progress := []string{}
	total := len(filesList)
	processed := 0

	if total == 0 {
		errorMessages = append(errorMessages, "No files selected.")
	}
	if outputDirectory == "" {
		errorMessages = append(errorMessages, "No output directory selected.")
	}

	// Store form values for repopulation
	formValues := gin.H{
		"DB":                 db,
		"Format":             format,
		"Action":             action,
		"Filter":             filter,
		"ConflictResolution": conflictResolution,
		"LogLevel":           logLevel,
		"Query":              query,
		"Recursive":          recursive,
	}

	logger.Log.Debug().Str("FileBotExecute", "called").Msg("FileBotExecute handler invoked")
	logger.Log.Debug().Strs("filesList", filesList).Str("outputDirectory", outputDirectory).Msg("FileBotExecute: files and output directory")
	logger.Log.Debug().Str("db", db).Str("format", format).Str("action", action).Str("filter", filter).Str("conflictResolution", conflictResolution).Str("logLevel", logLevel).Str("query", query).Bool("recursive", recursive).Msg("FileBotExecute: form values")
	isHTMX := c.GetHeader("HX-Request") != ""
	logger.Log.Debug().Bool("isHTMX", isHTMX).Msg("FileBotExecute: HTMX request detected")

	if len(errorMessages) == 0 {
		for i, file := range filesList {
			logger.Log.Debug().Str("file", file).Int("index", i).Int("total", total).Msg("FileBotExecute: processing file")
			args := []string{"-rename", file, "--db", db, "--action", action, "--conflict", conflictResolution, "--log", logLevel, "--output", outputDirectory, "-non-strict"}
			if format != "" {
				args = append(args, "--format", format)
			}
			if filter != "" {
				args = append(args, "--filter", filter)
			}
			if query != "" {
				args = append(args, "--q", query)
			}
			if recursive {
				args = append(args, "-r")
			}
			logger.Log.Debug().Strs("args", args).Msg("FileBotExecute: command args")
			cmd := exec.Command("filebot", args...)
			output, err := cmd.CombinedOutput()
			logger.Log.Debug().Str("output", string(output)).Err(err).Msg("FileBotExecute: command output and error")
			processed++
			if err != nil {
				errorMessages = append(errorMessages, "Error processing file '"+file+"': "+err.Error()+" Output: "+string(output))
				progress = append(progress, "❌ Error for "+file)
				continue
			}
			successMessages = append(successMessages, "Successfully processed file '"+file+"': "+string(output))
			progress = append(progress, "✅ Success for "+file)
		}
	}

	status := ""
	if processed > 0 {
		status = "Processed " + strconv.Itoa(processed) + " of " + strconv.Itoa(total) + " files."
	}

	// Only include status, progress, errors, and successes if there are any
	// This ensures that when the form is submitted again, previous messages are cleared
	data := gin.H{
		"FilesJSON":           filesJSON,
		"FilesList":           filesList,
		"OutputDirectoryJSON": outputDirectoryJSON,
		"OutputDirectory":     outputDirectory,
	}

	// Only add status, progress, errors, and successes if they have content
	if status != "" {
		data["Status"] = status
	}
	if len(progress) > 0 {
		data["Progress"] = progress
	}
	if len(errorMessages) > 0 {
		data["Errors"] = errorMessages
	}
	if len(successMessages) > 0 {
		data["Successes"] = successMessages
	}
	for k, v := range formValues {
		data[k] = v
	}
	if len(errorMessages) > 0 {
		data["Error"] = strings.Join(errorMessages, "\n")
	}
	RenderWithHTMX(c, "filebot_form.html", data, true)
}
