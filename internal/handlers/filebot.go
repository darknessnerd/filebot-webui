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
	plexRepo    repository.PlexServerRepositoryInterface
}

// NewFileBotHandler creates a new FileBot handler
func NewFileBotHandler(config *config.Config, db *database.DB, authService *auth.Service, delugeRepo *repository.DelugeServerRepository, plexRepo repository.PlexServerRepositoryInterface) *FileBotHandler {
	return &FileBotHandler{
		config:      config,
		db:          db,
		authService: authService,
		delugeRepo:  delugeRepo,
		plexRepo:    plexRepo,
	}
}

// FileBotForm serves the FileBot form and handles execution
func (h *FileBotHandler) FileBotForm(c *gin.Context) {
	// Detailed debug information for senior developers with consistent format
	filePath := c.Query("file_path")
	mode := c.Query("mode")
	torrentID := c.Query("torrent_id")
	torrentIDs := c.QueryArray("torrent_id") // Support multiple torrent IDs for bulk operations
	sourcePath := c.Query("source_path")

	logger.Log.Debug().
		Str("handler", "FileBotHandler.FileBotForm").
		Str("file_path", filePath).
		Str("source_path", sourcePath).
		Str("mode", mode).
		Str("torrent_id", torrentID).
		Strs("torrent_ids", torrentIDs).
		Msg("FileBotForm handler invoked")

	// For initial load, no files or output directory selected
	data := gin.H{
		"FilesJSON":           "",
		"FilesList":           nil,
		"OutputDirectoryJSON": "",
		"OutputDirectory":     "",
		"Status":              "",
		"Successes":           nil,
		"Action":              "test",     // Set default action to test
		"TorrentID":           torrentID,  // Pass the torrent ID to the template (for single torrent compatibility)
		"TorrentIDs":          torrentIDs, // Pass all torrent IDs for bulk operations
		"DeleteTorrent":       true,       // Default to true for the delete torrent checkbox
		"PlexServers":         nil,        // Initialize Plex servers list
		"SelectedPlexServer":  "",         // Initialize selected Plex server
	}

	// Get user to load Plex servers
	userObj, userExists := c.Get("user_obj")
	if userExists {
		if user, ok := userObj.(*models.User); ok && h.plexRepo != nil {
			// Load user's Plex servers
			plexServers, err := h.plexRepo.GetServersByUser(user.ID)
			if err == nil && len(plexServers) > 0 {
				data["PlexServers"] = plexServers
				logger.Log.Debug().Int("plex_servers_count", len(plexServers)).Msg("FileBotForm: Loaded Plex servers")

				// Set preferred server as default selection if exists
				for _, server := range plexServers {
					if server.Preferred {
						data["SelectedPlexServer"] = server.ID
						// Set Plex format preferences for JavaScript
						data["PlexMovieFormat"] = server.MovieFormat
						data["PlexSeriesFormat"] = server.SeriesFormat
						data["PlexAnimeFormat"] = server.AnimeFormat
						data["PlexMusicFormat"] = server.MusicFormat
						logger.Log.Debug().
							Int("preferred_server_id", server.ID).
							Str("server_name", server.Name).
							Str("movie_format", server.MovieFormat).
							Str("series_format", server.SeriesFormat).
							Str("anime_format", server.AnimeFormat).
							Str("music_format", server.MusicFormat).
							Msg("FileBotForm: Set preferred Plex server formats")
						break
					}
				}
			} else {
				logger.Log.Debug().Err(err).Int("user_id", user.ID).Msg("FileBotForm: No Plex servers found or error loading")
			}
		}
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

	// Determine which torrent IDs to process
	var targetTorrentIDs []string
	if len(torrentIDs) > 0 {
		targetTorrentIDs = torrentIDs
		logger.Log.Debug().Strs("torrent_ids", torrentIDs).Msg("FileBotForm: Processing multiple torrent IDs (bulk operation)")
	} else if torrentID != "" {
		targetTorrentIDs = []string{torrentID}
		logger.Log.Debug().Str("torrent_id", torrentID).Msg("FileBotForm: Processing single torrent ID")
	}

	// If we have torrent ID(s), fetch torrent info from Deluge
	if len(targetTorrentIDs) > 0 {
		logger.Log.Debug().Strs("torrent_ids", targetTorrentIDs).Msg("FileBotForm: Fetching torrent info from Deluge")
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

							var foundPaths []string
							var foundTorrentNames []string

							// Find the specific torrents
							for _, torrent := range torrents {
								for _, targetID := range targetTorrentIDs {
									if torrent.ID == targetID {
										// Construct the full path to the downloaded file
										fullPath := fmt.Sprintf("%s/%s", torrent.DownloadPath, torrent.Name)
										foundPaths = append(foundPaths, fullPath)
										foundTorrentNames = append(foundTorrentNames, torrent.Name)
										logger.Log.Debug().
											Str("torrent_id", torrent.ID).
											Str("torrent_name", torrent.Name).
											Str("download_path", torrent.DownloadPath).
											Str("full_path", fullPath).
											Msg("FileBotForm: Found matching torrent")
										break
									}
								}
							}

							if len(foundPaths) > 0 {
								// For backward compatibility, set filePath to the first found path
								if filePath == "" {
									filePath = foundPaths[0]
								}

								// Set up the files list for bulk processing
								if len(foundPaths) > 1 {
									// Multiple torrents found - set up for bulk processing
									filesJSON, _ := json.Marshal(foundPaths)
									data["FilesJSON"] = string(filesJSON)
									data["FilesList"] = foundPaths
									data["Status"] = fmt.Sprintf("Loaded %d torrents for bulk processing: %s", len(foundPaths), strings.Join(foundTorrentNames, ", "))
								} else {
									// Single torrent - maintain existing behavior
									data["FilesJSON"] = fmt.Sprintf("[\"%s\"]", foundPaths[0])
									data["FilesList"] = foundPaths
									data["Status"] = fmt.Sprintf("File loaded from torrent: %s", foundTorrentNames[0])
								}
							} else {
								logger.Log.Debug().Strs("torrent_ids", targetTorrentIDs).Msg("FileBotForm: No matching torrents found with provided IDs")
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

	// If we have a file path and no files were loaded from torrents, add it to the files list
	if filePath != "" && data["FilesList"] == nil {
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
		"DeleteTorrent":       true,      // Default to true for the delete torrent checkbox
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
		Str("delete_torrent", c.PostForm("delete_torrent")).
		Str("files_json_length", strconv.Itoa(len(c.PostForm("files")))).
		Str("output_directory", c.PostForm("outputDirectory")).
		Str("torrent_id", c.PostForm("torrent_id")).
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
	deleteTorrent := c.PostForm("delete_torrent") == "true"
	filesJSON := c.PostForm("files")
	outputDirectoryJSON := c.PostForm("outputDirectory")
	torrentID := c.PostForm("torrent_id")

	// Handle multiple torrent IDs for bulk operations
	torrentIDs := c.PostFormArray("torrent_ids")
	if len(torrentIDs) == 0 && torrentID != "" {
		// Fallback to single torrent ID for backward compatibility
		torrentIDs = []string{torrentID}
	}

	logger.Log.Debug().
		Str("torrent_id", torrentID).
		Strs("torrent_ids", torrentIDs).
		Int("torrent_ids_count", len(torrentIDs)).
		Msg("FileBotExecute: Parsed torrent IDs for deletion")

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
		"TorrentID":          torrentID,
		"DeleteTorrent":      deleteTorrent,
	}

	isHTMX := c.GetHeader("HX-Request") != ""
	logger.Log.Debug().
		Bool("isHTMX", isHTMX).
		Bool("has_validation_errors", len(errorMessages) > 0).
		Int("files_count", total).
		Bool("has_output_dir", outputDirectory != "").
		Bool("delete_torrent", deleteTorrent).
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

		// Debug torrent deletion conditions
		logger.Log.Debug().
			Bool("delete_torrent", deleteTorrent).
			Strs("torrent_ids", torrentIDs).
			Int("torrent_ids_count", len(torrentIDs)).
			Int("error_messages_count", len(errorMessages)).
			Int("processed", processed).
			Int("total", total).
			Str("action", action).
			Msg("FileBotExecute: Checking torrent deletion conditions")

		// Delete torrents if:
		// 1. The delete_torrent checkbox was selected
		// 2. Torrent IDs were provided
		// 3. All files were processed successfully (no errors)
		// 4. The action was "move" (only makes sense to delete after moving, not for test/copy/symlink)
		if deleteTorrent && len(torrentIDs) > 0 && len(errorMessages) == 0 && processed == total && total > 0 && action == "move" {
			logger.Log.Info().
				Strs("torrent_ids", torrentIDs).
				Int("torrent_count", len(torrentIDs)).
				Msg("FileBotExecute: Attempting to delete torrents after successful move operation")

			// Get the deluge service from context
			delugeService, exists := c.Get("deluge_service")
			if !exists {
				logger.Log.Warn().Msg("FileBotExecute: deluge_service not found in context, cannot delete torrents")
			} else if delugeHandler, ok := delugeService.(*DelugeHandler); ok {
				// Get user to get preferred Deluge server
				userObj, _ := c.Get("user_obj")
				user, ok := userObj.(*models.User)

				if ok && h.delugeRepo != nil {
					// Get preferred Deluge server
					server, err := h.delugeRepo.GetPreferredDelugeServer(user.ID)
					if err != nil {
						logger.Log.Warn().
							Err(err).
							Int("user_id", user.ID).
							Msg("FileBotExecute: Error getting preferred Deluge server, cannot delete torrents")
					} else if server != nil {
						// Track deletion results
						deletedCount := 0
						failedCount := 0

						// Remove each torrent
						for _, currentTorrentID := range torrentIDs {
							logger.Log.Debug().
								Str("torrent_id", currentTorrentID).
								Msg("FileBotExecute: Attempting to delete individual torrent")

							err = delugeHandler.removeTorrentFromServer(server, currentTorrentID, true)
							if err != nil {
								failedCount++
								logger.Log.Warn().
									Err(err).
									Str("torrent_id", currentTorrentID).
									Msg("FileBotExecute: Error deleting torrent after move operation")
							} else {
								deletedCount++
								logger.Log.Info().
									Str("torrent_id", currentTorrentID).
									Msg("FileBotExecute: Successfully deleted torrent after move operation")
							}
						}

						// Report deletion results
						if deletedCount > 0 && failedCount == 0 {
							if deletedCount == 1 {
								successMessages = append(successMessages, "Torrent was successfully removed from Deluge after files were moved.")
							} else {
								successMessages = append(successMessages, fmt.Sprintf("All %d torrents were successfully removed from Deluge after files were moved.", deletedCount))
							}
						} else if deletedCount > 0 && failedCount > 0 {
							successMessages = append(successMessages, fmt.Sprintf("%d of %d torrents were removed successfully. %d failed to delete.", deletedCount, len(torrentIDs), failedCount))
						} else if failedCount > 0 {
							successMessages = append(successMessages, "Note: Files were moved successfully, but automatic torrent removal failed. You may need to remove the torrents manually.")
						}

						logger.Log.Info().
							Int("deleted_count", deletedCount).
							Int("failed_count", failedCount).
							Int("total_torrents", len(torrentIDs)).
							Msg("FileBotExecute: Torrent deletion completed")
					}
				}
			} else {
				logger.Log.Warn().Msg("FileBotExecute: deluge_service type assertion failed")
			}
		} else {
			// Log why torrent deletion was skipped
			logger.Log.Debug().
				Bool("delete_torrent_checked", deleteTorrent).
				Bool("torrent_ids_provided", len(torrentIDs) > 0).
				Bool("no_errors", len(errorMessages) == 0).
				Bool("all_processed", processed == total).
				Bool("has_files", total > 0).
				Bool("is_move_action", action == "move").
				Msg("FileBotExecute: Torrent deletion skipped - conditions not met")
		}
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
		"TorrentID":           torrentID, // Preserve the torrent ID
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

// GetPlexServerFormats returns the format configuration for a specific Plex server
func (h *FileBotHandler) GetPlexServerFormats(c *gin.Context) {
	serverIDStr := c.Param("id")
	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		logger.Log.Debug().Str("server_id", serverIDStr).Err(err).Msg("GetPlexServerFormats: Invalid server ID")
		c.JSON(400, gin.H{"error": "Invalid server ID"})
		return
	}

	userObj, userExists := c.Get("user_obj")
	if !userExists {
		logger.Log.Debug().Msg("GetPlexServerFormats: No user object found")
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	user, ok := userObj.(*models.User)
	if !ok {
		logger.Log.Debug().Msg("GetPlexServerFormats: Invalid user object")
		c.JSON(401, gin.H{"error": "Invalid user"})
		return
	}

	if h.plexRepo == nil {
		logger.Log.Debug().Msg("GetPlexServerFormats: Plex repository not available")
		c.JSON(500, gin.H{"error": "Plex service not available"})
		return
	}

	// Get all user's servers to verify ownership
	servers, err := h.plexRepo.GetServersByUser(user.ID)
	if err != nil {
		logger.Log.Debug().Err(err).Int("user_id", user.ID).Msg("GetPlexServerFormats: Error loading user servers")
		c.JSON(500, gin.H{"error": "Error loading servers"})
		return
	}

	// Find the specific server
	var selectedServer *models.PlexServer
	for _, server := range servers {
		if server.ID == serverID {
			selectedServer = &server
			break
		}
	}

	if selectedServer == nil {
		logger.Log.Debug().Int("server_id", serverID).Int("user_id", user.ID).Msg("GetPlexServerFormats: Server not found or not owned by user")
		c.JSON(404, gin.H{"error": "Server not found"})
		return
	}

	logger.Log.Debug().
		Int("server_id", serverID).
		Str("server_name", selectedServer.Name).
		Str("movie_format", selectedServer.MovieFormat).
		Str("series_format", selectedServer.SeriesFormat).
		Str("anime_format", selectedServer.AnimeFormat).
		Str("music_format", selectedServer.MusicFormat).
		Msg("GetPlexServerFormats: Returning server formats")

	c.JSON(200, gin.H{
		"movie_format":  selectedServer.MovieFormat,
		"series_format": selectedServer.SeriesFormat,
		"anime_format":  selectedServer.AnimeFormat,
		"music_format":  selectedServer.MusicFormat,
	})
}
