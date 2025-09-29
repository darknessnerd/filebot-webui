package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"
)

// FileBotHandler contains handler methods for FileBot operations
type FileBotHandler struct {
	config      *config.Config
	db          *database.DB
	authService *auth.Service
	delugeRepo  *repository.DelugeServerRepository
}

// NewFileBotHandler creates a new FileBot handler
func NewFileBotHandler(config *config.Config, db *database.DB, authService *auth.Service, delugeRepo *repository.DelugeServerRepository) *FileBotHandler {
	return &FileBotHandler{
		config:      config,
		db:          db,
		authService: authService,
		delugeRepo:  delugeRepo,
	}
}

// FileBotForm serves the FileBot form and handles execution
func (h *FileBotHandler) FileBotForm(c *gin.Context) {
	// Detailed debug information for senior developers with consistent format
	filePath := c.Query("file_path")
	mode := c.Query("mode")
	torrentID := c.Query("torrent_id")
	sourcePath := c.Query("source_path")

	logger.Log.Debug().
		Str("handler", "FileBotHandler.FileBotForm").
		Str("file_path", filePath).
		Str("source_path", sourcePath).
		Str("mode", mode).
		Str("torrent_id", torrentID).
		Msg("FileBotForm handler invoked")

	// For initial load, no files or output directory selected
	data := gin.H{
		"FilesJSON":           "",
		"FilesList":           nil,
		"OutputDirectoryJSON": "",
		"OutputDirectory":     "",
		"Status":              "",
		"Successes":           nil,
		"Action":              "test",    // Set default action to test
		"TorrentID":           torrentID, // Pass the torrent ID to the template
	}

	// If mode is specified, set it as the action
	if mode != "" {
		logger.Log.Debug().Str("mode", mode).Msg("FileBotForm setting action from mode parameter")
		data["Action"] = mode
	}

	// Use source_path parameter if provided (from Deluge handler redirect)
	if sourcePath != "" {
		filePath = sourcePath
		logger.Log.Debug().Str("source_path", sourcePath).Msg("FileBotForm using source_path parameter")
	}

	// If we have a torrent_id, fetch torrent info from Deluge
	if torrentID != "" {
		logger.Log.Debug().Str("torrent_id", torrentID).Msg("FileBotForm: Fetching torrent info from Deluge")
		userObj, _ := c.Get("user_obj")
		user, ok := userObj.(*models.User)

		if ok && h.delugeRepo != nil {
			// Get preferred Deluge server
			server, err := h.delugeRepo.GetPreferredDelugeServer(user.ID)
			if err != nil {
				logger.Log.Debug().Err(err).Int("user_id", user.ID).Msg("FileBotForm: Error getting preferred Deluge server")
			} else if server != nil {
				logger.Log.Debug().Str("server_name", server.Name).Str("server_host", server.Host).Int("server_port", server.Port).Msg("FileBotForm: Using Deluge server")

				// Get Deluge service from the context (it's set in routes.go)
				delugeService, exists := c.Get("deluge_service")
				if !exists {
					logger.Log.Debug().Msg("FileBotForm: deluge_service not found in context")
				} else {
					// Type assertion to get the actual deluge service
					if delugeHandler, ok := delugeService.(*DelugeHandler); ok {
						// Get torrent info from Deluge
						torrents, err := delugeHandler.getTorrents(server)
						if err != nil {
							logger.Log.Debug().Err(err).Msg("FileBotForm: Error getting torrents from Deluge")
						} else {
							logger.Log.Debug().Int("torrents_count", len(torrents)).Msg("FileBotForm: Got torrents from Deluge")
							// Find the specific torrent
							for _, torrent := range torrents {
								if torrent.ID == torrentID {
									// Construct the full path to the downloaded file
									filePath = fmt.Sprintf("%s/%s", torrent.DownloadPath, torrent.Name)
									logger.Log.Debug().
										Str("torrent_id", torrent.ID).
										Str("torrent_name", torrent.Name).
										Str("download_path", torrent.DownloadPath).
										Str("full_path", filePath).
										Msg("FileBotForm: Found matching torrent")
									break
								}
							}
							if filePath == "" {
								logger.Log.Debug().Str("torrent_id", torrentID).Msg("FileBotForm: No matching torrent found with provided ID")
							}
						}
					} else {
						logger.Log.Debug().Msg("FileBotForm: deluge_service type assertion failed")
					}
				}
			} else {
				logger.Log.Debug().Int("user_id", user.ID).Msg("FileBotForm: No preferred Deluge server found")
			}
		} else {
			logger.Log.Debug().Bool("user_ok", ok).Bool("repo_nil", h.delugeRepo == nil).Msg("FileBotForm: Invalid user or missing repository")
		}
	}

	// If we have a file path, add it to the files list
	if filePath != "" {
		data["FilesJSON"] = fmt.Sprintf("[\"%s\"]", filePath)
		data["FilesList"] = []string{filePath}
		data["Status"] = "File loaded from parameters"
	}

	RenderWithHTMX(c, "filebot_form.html", data, true)
}

// FileBotSelection handles updates from the directory browser to the FileBot form
func (h *FileBotHandler) FileBotSelection(c *gin.Context) {
	// Detailed debug information for senior developers
	logger.Log.Debug().
		Str("handler", "FileBotHandler.FileBotSelection").
		Str("files_json", c.PostForm("files")).
		Str("output_directory_json", c.PostForm("outputDirectory")).
		Str("action", c.PostForm("action")).
		Str("torrent_id", c.PostForm("torrent_id")).
		Msg("FileBotSelection handler invoked")

	filesJSON := c.PostForm("files")
	outputDirectoryJSON := c.PostForm("outputDirectory")
	torrentID := c.PostForm("torrent_id")
	action := c.PostForm("action")

	if action == "" {
		action = "test" // Default to test if not specified
	}

	// Preserve selected files even when only output directory is changed
	filesList := []string{}
	if filesJSON != "" {
		if strings.HasPrefix(filesJSON, "[") {
			err := json.Unmarshal([]byte(filesJSON), &filesList)
			if err != nil {
				logger.Log.Debug().Err(err).Str("filesJSON", filesJSON).Msg("FileBotSelection: Error unmarshalling files JSON")
			} else {
				logger.Log.Debug().Int("files_count", len(filesList)).Msg("FileBotSelection: Successfully parsed JSON file list")
			}
		} else {
			filesList = strings.Split(filesJSON, ",")
			logger.Log.Debug().Int("files_count", len(filesList)).Msg("FileBotSelection: Parsed comma-separated file list")
		}
	} else {
		logger.Log.Debug().Msg("FileBotSelection: No files selected")
	}

	outputDirectory := ""
	if outputDirectoryJSON != "" {
		outputDirectory = outputDirectoryJSON
		logger.Log.Debug().Str("outputDirectory", outputDirectory).Msg("FileBotSelection: Output directory set")
	} else {
		logger.Log.Debug().Msg("FileBotSelection: No output directory selected")
	}

	data := gin.H{
		"FilesJSON":           filesJSON,
		"FilesList":           filesList,
		"OutputDirectoryJSON": outputDirectoryJSON,
		"OutputDirectory":     outputDirectory,
		"Status":              "",
		"Errors":              nil,
		"Successes":           nil,
		"Action":              action,    // Preserve the action
		"TorrentID":           torrentID, // Preserve the torrent ID
	}

	logger.Log.Debug().
		Int("files_count", len(filesList)).
		Bool("has_output_dir", outputDirectory != "").
		Msg("FileBotSelection: Rendering form with selection data")

	RenderWithHTMX(c, "filebot_form.html", data, true)
}

// FileBotExecute runs the FileBot command for selected files and output directory
func (h *FileBotHandler) FileBotExecute(c *gin.Context) {
	// Detailed debug information for senior developers
	logger.Log.Debug().
		Str("handler", "FileBotHandler.FileBotExecute").
		Str("db", c.PostForm("db")).
		Str("format", c.PostForm("format")).
		Str("action", c.PostForm("action")).
		Str("filter", c.PostForm("filter")).
		Str("conflict_resolution", c.PostForm("conflict_resolution")).
		Str("log_level", c.PostForm("log_level")).
		Str("query", c.PostForm("query")).
		Str("recursive", c.PostForm("recursive")).
		Str("files_json_length", strconv.Itoa(len(c.PostForm("files")))).
		Str("output_directory", c.PostForm("outputDirectory")).
		Str("request_id", c.GetHeader("X-Request-ID")).
		Str("client_ip", c.ClientIP()).
		Msg("FileBotExecute handler invoked")

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
			err := json.Unmarshal([]byte(filesJSON), &filesList)
			if err != nil {
				logger.Log.Debug().
					Err(err).
					Str("filesJSON", filesJSON).
					Msg("FileBotExecute: Error unmarshalling files JSON")
			} else {
				logger.Log.Debug().
					Int("files_count", len(filesList)).
					Msg("FileBotExecute: Successfully parsed JSON file list")
			}
		} else {
			filesList = strings.Split(filesJSON, ",")
			logger.Log.Debug().
				Int("files_count", len(filesList)).
				Msg("FileBotExecute: Parsed comma-separated file list")
		}
	} else {
		logger.Log.Debug().Msg("FileBotExecute: No files provided in request")
	}

	outputDirectory := outputDirectoryJSON

	successMessages := []string{}
	errorMessages := []string{}
	progress := []string{}
	total := len(filesList)
	processed := 0

	// Validate required parameters
	if total == 0 {
		logger.Log.Debug().Msg("FileBotExecute: No files selected - validation error")
		errorMessages = append(errorMessages, "No files selected.")
	}
	if outputDirectory == "" {
		logger.Log.Debug().Msg("FileBotExecute: No output directory selected - validation error")
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

	isHTMX := c.GetHeader("HX-Request") != ""
	logger.Log.Debug().
		Bool("isHTMX", isHTMX).
		Bool("has_validation_errors", len(errorMessages) > 0).
		Int("files_count", total).
		Bool("has_output_dir", outputDirectory != "").
		Msg("FileBotExecute: Request validation")

	if len(errorMessages) == 0 {
		logger.Log.Debug().
			Int("total_files", total).
			Msg("FileBotExecute: Starting file processing")

		for i, file := range filesList {
			logger.Log.Debug().
				Str("file", file).
				Int("index", i).
				Int("total", total).
				Int("processed", processed).
				Msg("FileBotExecute: Processing file")

			// Build command arguments
			args := []string{"-rename", file, "--db", db, "--action", action, "--conflict", conflictResolution, "--log", logLevel, "--output", outputDirectory, "-non-strict"}

			// Add optional parameters if provided
			if format != "" {
				args = append(args, "--format", format)
				logger.Log.Debug().Str("format", format).Msg("FileBotExecute: Using custom format")
			}
			if filter != "" {
				args = append(args, "--filter", filter)
				logger.Log.Debug().Str("filter", filter).Msg("FileBotExecute: Using filter")
			}
			if query != "" {
				args = append(args, "--q", query)
				logger.Log.Debug().Str("query", query).Msg("FileBotExecute: Using query")
			}
			if recursive {
				args = append(args, "-r")
				logger.Log.Debug().Msg("FileBotExecute: Using recursive mode")
			}

			// Log the full command for troubleshooting
			cmdStr := "filebot " + strings.Join(args, " ")
			logger.Log.Debug().
				Str("command", cmdStr).
				Strs("args", args).
				Msg("FileBotExecute: Executing command")

			// Execute FileBot command
			start := time.Now()
			cmd := exec.Command("filebot", args...)
			output, err := cmd.CombinedOutput()
			execDuration := time.Since(start)

			// Log command execution details
			logger.Log.Debug().
				Str("file", file).
				Str("output", string(output)).
				Dur("execution_time_ms", execDuration).
				Err(err).
				Msg("FileBotExecute: Command execution completed")

			processed++
			if err != nil {
				errorMessage := "Error processing file '" + file + "': " + err.Error() + " Output: " + string(output)
				errorMessages = append(errorMessages, errorMessage)
				progress = append(progress, "❌ Error for "+file)
				logger.Log.Debug().
					Str("file", file).
					Err(err).
					Str("error_message", errorMessage).
					Msg("FileBotExecute: Command failed")
				continue
			}

			successMessage := "Successfully processed file '" + file + "': " + string(output)
			successMessages = append(successMessages, successMessage)
			progress = append(progress, "✅ Success for "+file)
			logger.Log.Debug().
				Str("file", file).
				Str("success_message", successMessage).
				Msg("FileBotExecute: Command succeeded")
		}

		logger.Log.Debug().
			Int("total", total).
			Int("processed", processed).
			Int("success_count", len(successMessages)).
			Int("error_count", len(errorMessages)).
			Msg("FileBotExecute: File processing completed")
	}

	status := ""
	if processed > 0 {
		status = "Processed " + strconv.Itoa(processed) + " of " + strconv.Itoa(total) + " files."
		logger.Log.Debug().Str("status", status).Msg("FileBotExecute: Generated status message")
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

	logger.Log.Debug().
		Int("files_count", len(filesList)).
		Bool("has_errors", len(errorMessages) > 0).
		Bool("has_successes", len(successMessages) > 0).
		Int("progress_items", len(progress)).
		Bool("has_status", status != "").
		Msg("FileBotExecute: Rendering result form")

	RenderWithHTMX(c, "filebot_form.html", data, true)
}
