package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"

	"github.com/gin-gonic/gin"
)

type DelugeHandler struct {
	config  *config.Config
	db      *database.DB
	authSvc *auth.Service
	repo    repository.DelugeServerRepositoryInterface
}

// NewDelugeHandler creates a new Deluge handler
func NewDelugeHandler(config *config.Config, db *database.DB, authSvc *auth.Service, repo repository.DelugeServerRepositoryInterface) *DelugeHandler {
	return &DelugeHandler{
		config:  config,
		db:      db,
		authSvc: authSvc,
		repo:    repo,
	}
}

// RenderDelugeConfigurePage renders the Deluge configuration page
func (h *DelugeHandler) RenderDelugeConfigurePage(c *gin.Context) {
	userObj, _ := c.Get("user_obj")

	c.HTML(http.StatusOK, "deluge_configure.html", gin.H{
		"user": userObj,
	})
}

// RenderDelugeServersHTMX renders the Deluge servers list
func (h *DelugeHandler) RenderDelugeServersHTMX(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	logger.Log.Debug().Msgf("[HTMX][RenderDelugeServersHTMX] Called for user: %s", user.Name)

	servers, err := h.repo.GetServersByUser(user.ID)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderDelugeServersHTMX] Failed to get servers for user %s: %v", user.Name, err)
		c.String(http.StatusInternalServerError, "Failed to get Deluge servers")
		return
	}

	preferredServer, err := h.repo.GetPreferredDelugeServer(user.ID)
	var preferredServerID = -1
	if err == nil && preferredServer != nil {
		preferredServerID = preferredServer.ID
		logger.Log.Debug().Msgf("[HTMX][RenderDelugeServersHTMX] Preferred server for user %s: %d", user.Name, preferredServerID)
	} else if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderDelugeServersHTMX] Failed to get preferred server for user %s: %v", user.Name, err)
	}

	c.HTML(http.StatusOK, "deluge_servers.html", gin.H{
		"servers":            servers,
		"preferredServer":    preferredServerID,
		"hasPreferredServer": preferredServerID != -1,
	})
}

// AddDelugeServer adds a new Deluge server
func (h *DelugeHandler) AddDelugeServer(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	name := c.PostForm("name")
	host := c.PostForm("host")
	portStr := c.PostForm("port")
	username := c.PostForm("username")
	password := c.PostForm("password")
	protocol := c.PostForm("protocol")
	setPreferred := c.PostForm("set_preferred") == "true"

	if name == "" || host == "" || portStr == "" || username == "" || password == "" {
		c.String(http.StatusBadRequest, "Missing required fields")
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid port number")
		return
	}

	// Create server object
	server := &models.DelugeServer{
		Name:      name,
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		Protocol:  protocol,
		URI:       fmt.Sprintf("%s://%s:%d", protocol, host, port),
		CreatedAt: time.Now(),
		LastSeenAt: time.Now(),
		Preferred: setPreferred,
	}

	// Test connection to the server
	connected, err := h.testDelugeConnection(server)
	if err != nil {
		server.Connected = false
		server.LastError = err.Error()
		server.LastErrorAt = time.Now()
	} else {
		server.Connected = connected
		if !connected {
			server.LastError = "Connection test failed"
			server.LastErrorAt = time.Now()
		}
	}

	// Save server to database
	err = h.repo.UpsertDelugeServer(user, server)
	if err != nil {
		logger.Log.Error().Msgf("[AddDelugeServer] Failed to save server for user %s: %v", user.Name, err)
		c.String(http.StatusInternalServerError, "Failed to save Deluge server")
		return
	}

	// Set as preferred if requested
	if setPreferred {
		err = h.repo.SetPreferredDelugeServer(user.ID, server.ID)
		if err != nil {
			logger.Log.Error().Msgf("[AddDelugeServer] Failed to set preferred server for user %s: %v", user.Name, err)
		}
	}

	// Redirect to servers list
	c.Redirect(http.StatusSeeOther, "/deluge/servers/htmx")
}

// SetPreferredDelugeServer sets the preferred Deluge server
func (h *DelugeHandler) SetPreferredDelugeServer(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	serverIDStr := c.PostForm("server_id")

	logger.Log.Debug().Msgf("[HTMX][SetPreferredDelugeServer] Called for user: %s, serverID: %s", user.Name, serverIDStr)

	if serverIDStr == "" {
		logger.Log.Error().Msgf("[HTMX][SetPreferredDelugeServer] Missing server ID for user: %s", user.Name)
		c.String(http.StatusBadRequest, "Missing server ID")
		return
	}

	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][SetPreferredDelugeServer] Invalid server ID for user: %s, error: %v", user.Name, err)
		c.String(http.StatusBadRequest, "Invalid server ID")
		return
	}

	// Update preferred server in deluge_servers table
	err = h.repo.SetPreferredDelugeServer(user.ID, serverID)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][SetPreferredDelugeServer] Failed to set preferred server for user: %s, error: %v", user.Name, err)
		c.String(http.StatusInternalServerError, "Failed to set preferred server")
		return
	}

	logger.Log.Debug().Msgf("[HTMX][SetPreferredDelugeServer] Preferred server updated for user: %s, serverID: %d", user.Name, serverID)
	c.String(http.StatusOK, "Preferred server updated")
}

// DeleteDelugeServer deletes a Deluge server
func (h *DelugeHandler) DeleteDelugeServer(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	serverIDStr := c.PostForm("server_id")

	if serverIDStr == "" {
		c.String(http.StatusBadRequest, "Missing server ID")
		return
	}

	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid server ID")
		return
	}

	err = h.repo.DeleteServer(serverID, user.ID)
	if err != nil {
		logger.Log.Error().Msgf("[DeleteDelugeServer] Failed to delete server for user %s: %v", user.Name, err)
		c.String(http.StatusInternalServerError, "Failed to delete Deluge server")
		return
	}

	c.Redirect(http.StatusSeeOther, "/deluge/servers/htmx")
}

// TestDelugeConnection tests the connection to a Deluge server
func (h *DelugeHandler) TestDelugeConnection(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	serverIDStr := c.PostForm("server_id")

	if serverIDStr == "" {
		c.String(http.StatusBadRequest, "Missing server ID")
		return
	}

	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid server ID")
		return
	}

	// Get server from database
	servers, err := h.repo.GetServersByUser(user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get Deluge servers")
		return
	}

	var server *models.DelugeServer
	for _, s := range servers {
		if s.ID == serverID {
			server = &s
			break
		}
	}

	if server == nil {
		c.String(http.StatusNotFound, "Server not found")
		return
	}

	// Test connection
	connected, err := h.testDelugeConnection(server)
	if err != nil {
		logger.Log.Error().Msgf("[TestDelugeConnection] Connection test failed for server %s: %v", server.Name, err)
		h.repo.UpdateServerStatus(server.ID, false, err.Error())
		c.String(http.StatusInternalServerError, fmt.Sprintf("Connection test failed: %v", err))
		return
	}

	if !connected {
		h.repo.UpdateServerStatus(server.ID, false, "Connection test failed")
		c.String(http.StatusBadGateway, "Connection test failed")
		return
	}

	h.repo.UpdateServerStatus(server.ID, true, "")
	c.String(http.StatusOK, "Connection test successful")
}

// testDelugeConnection tests the connection to a Deluge server
func (h *DelugeHandler) testDelugeConnection(server *models.DelugeServer) (bool, error) {
	// Deluge Web API uses JSON-RPC
	// First, we need to authenticate
	authPayload := map[string]interface{}{
		"method": "auth.login",
		"params": []string{server.Password},
		"id":     1,
	}

	jsonData, err := json.Marshal(authPayload)
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("authentication failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var authResponse map[string]interface{}
	err = json.Unmarshal(body, &authResponse)
	if err != nil {
		return false, err
	}

	// Check if authentication was successful
	result, ok := authResponse["result"].(bool)
	if !ok || !result {
		return false, fmt.Errorf("authentication failed: %v", authResponse["error"])
	}

	// Get session cookie
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "_session_id" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		return false, fmt.Errorf("no session cookie found")
	}

	// Now test a simple API call to verify the connection
	client := &http.Client{}
	infoPayload := map[string]interface{}{
		"method": "web.connected",
		"params": []interface{}{},
		"id":     2,
	}

	jsonData, err = json.Marshal(infoPayload)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var infoResponse map[string]interface{}
	err = json.Unmarshal(body, &infoResponse)
	if err != nil {
		return false, err
	}

	// Check if the API call was successful
	connected, ok := infoResponse["result"].(bool)
	if !ok {
		return false, fmt.Errorf("API call failed: %v", infoResponse["error"])
	}

	return connected, nil
}

// RenderDelugeHomePage renders the Deluge home page
func (h *DelugeHandler) RenderDelugeHomePage(c *gin.Context) {
	userObj, _ := c.Get("user_obj")

	c.HTML(http.StatusOK, "deluge_home.html", gin.H{
		"user": userObj,
	})
}

// GetTorrentsHTMX returns the list of torrents for the HTMX widget
func (h *DelugeHandler) GetTorrentsHTMX(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Get torrents from the server
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to get torrents: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// GetServerStatusHTMX returns the server status for the HTMX widget
func (h *DelugeHandler) GetServerStatusHTMX(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_server_status.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Get server status
	status, err := h.getServerStatus(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_server_status.html", gin.H{
			"error": fmt.Sprintf("Failed to get server status: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_server_status.html", gin.H{
		"server": status,
	})
}

// AddTorrent adds a new torrent to Deluge
func (h *DelugeHandler) AddTorrent(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	torrentURL := c.PostForm("torrent_url")
	downloadLocation := c.PostForm("download_location")

	if torrentURL == "" {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "Torrent URL is required",
		})
		return
	}

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Add torrent
	err = h.addTorrentToServer(server, torrentURL, downloadLocation)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to add torrent: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Torrent added, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// PauseTorrent pauses a torrent
func (h *DelugeHandler) PauseTorrent(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	torrentID := c.PostForm("torrent_id")

	if torrentID == "" {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "Torrent ID is required",
		})
		return
	}

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Pause torrent
	err = h.pauseTorrentOnServer(server, torrentID)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to pause torrent: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Torrent paused, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// ResumeTorrent resumes a paused torrent
func (h *DelugeHandler) ResumeTorrent(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	torrentID := c.PostForm("torrent_id")

	if torrentID == "" {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "Torrent ID is required",
		})
		return
	}

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Resume torrent
	err = h.resumeTorrentOnServer(server, torrentID)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to resume torrent: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Torrent resumed, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// RemoveTorrent removes a torrent
func (h *DelugeHandler) RemoveTorrent(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	torrentID := c.PostForm("torrent_id")
	removeData := c.PostForm("remove_data") == "true"

	if torrentID == "" {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "Torrent ID is required",
		})
		return
	}

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Remove torrent
	err = h.removeTorrentFromServer(server, torrentID, removeData)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to remove torrent: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Torrent removed, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// PauseAllTorrents pauses all torrents
func (h *DelugeHandler) PauseAllTorrents(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Pause all torrents
	err = h.pauseAllTorrentsOnServer(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to pause all torrents: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("All torrents paused, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// ResumeAllTorrents resumes all torrents
func (h *DelugeHandler) ResumeAllTorrents(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	// Get preferred server
	server, err := h.repo.GetPreferredDelugeServer(user.ID)
	if err != nil || server == nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": "No preferred Deluge server found. Please configure one in the Deluge Configuration page.",
		})
		return
	}

	// Resume all torrents
	err = h.resumeAllTorrentsOnServer(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("Failed to resume all torrents: %v", err),
		})
		return
	}

	// Get updated list of torrents
	torrents, err := h.getTorrents(server)
	if err != nil {
		c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
			"error": fmt.Sprintf("All torrents resumed, but failed to refresh torrent list: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "deluge_torrents.html", gin.H{
		"torrents": torrents,
	})
}

// Helper methods for interacting with the Deluge API

// getTorrents gets the list of torrents from the Deluge server
func (h *DelugeHandler) getTorrents(server *models.DelugeServer) ([]models.DelugeTorrent, error) {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return nil, err
	}

	// Get torrents
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	// Request torrent list with all fields
	fields := []string{
		"name", "state", "progress", "download_payload_rate", "upload_payload_rate",
		"eta", "total_size", "total_done", "total_uploaded", "ratio", "num_seeds",
		"num_peers", "time_added", "completed_time", "download_location", "label",
		"is_finished", "is_auto_managed", "private", "sequential_download", "super_seeding",
	}
	params := []interface{}{fields, map[string]interface{}{}}
	payload := map[string]interface{}{
		"method": "web.update_ui",
		"params": params,
		"id":     3,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the response manually to avoid type assertion issues
	var responseMap map[string]interface{}
	err = json.Unmarshal(body, &responseMap)
	if err != nil {
		return nil, err
	}

	// Convert response to DelugeTorrent objects
	torrents := []models.DelugeTorrent{}

	// Check if result exists and is a map
	resultVal, resultExists := responseMap["result"]
	if !resultExists {
		return torrents, nil
	}

	resultMap, ok := resultVal.(map[string]interface{})
	if !ok {
		return torrents, nil
	}

	// Check if torrents exists and is a map
	torrentsVal, torrentsExist := resultMap["torrents"]
	if !torrentsExist {
		return torrents, nil
	}

	torrentsMap, ok := torrentsVal.(map[string]interface{})
	if !ok {
		return torrents, nil
	}

	// Process each torrent
	for id, data := range torrentsMap {
		torrentData, ok := data.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract values with type assertions and default values
		name, _ := torrentData["name"].(string)
		state, _ := torrentData["state"].(string)
		progress, _ := torrentData["progress"].(float64)
		downloadRate, _ := torrentData["download_payload_rate"].(float64)
		uploadRate, _ := torrentData["upload_payload_rate"].(float64)
		eta, _ := torrentData["eta"].(float64)
		size, _ := torrentData["total_size"].(float64)
		downloaded, _ := torrentData["total_done"].(float64)
		uploaded, _ := torrentData["total_uploaded"].(float64)
		ratio, _ := torrentData["ratio"].(float64)
		seeds, _ := torrentData["num_seeds"].(float64)
		peers, _ := torrentData["num_peers"].(float64)
		addedOn, _ := torrentData["time_added"].(float64)
		completedOn, _ := torrentData["completed_time"].(float64)
		downloadPath, _ := torrentData["download_location"].(string)
		label, _ := torrentData["label"].(string)
		isFinished, _ := torrentData["is_finished"].(bool)
		isAutoManaged, _ := torrentData["is_auto_managed"].(bool)
		isPrivate, _ := torrentData["private"].(bool)
		isSequential, _ := torrentData["sequential_download"].(bool)
		isSuperSeeding, _ := torrentData["super_seeding"].(bool)

		torrent := models.DelugeTorrent{
			ID:             id,
			Name:           name,
			State:          state,
			Progress:       progress,
			DownloadSpeed:  int64(downloadRate),
			UploadSpeed:    int64(uploadRate),
			ETA:            int64(eta),
			Size:           int64(size),
			Downloaded:     int64(downloaded),
			Uploaded:       int64(uploaded),
			Ratio:          ratio,
			Seeds:          int(seeds),
			Peers:          int(peers),
			AddedOn:        time.Unix(int64(addedOn), 0),
			CompletedOn:    time.Unix(int64(completedOn), 0),
			DownloadPath:   downloadPath,
			Label:          label,
			IsFinished:     isFinished,
			IsAutoManaged:  isAutoManaged,
			IsPrivate:      isPrivate,
			IsSequential:   isSequential,
			IsSuperSeeding: isSuperSeeding,
			ServerID:       server.ID,
		}

		torrents = append(torrents, torrent)
	}

	return torrents, nil
}

// getServerStatus gets the status of the Deluge server
func (h *DelugeHandler) getServerStatus(server *models.DelugeServer) (*models.DelugeServerStatus, error) {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return nil, err
	}

	// Get server status
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	// Request session status
	payload := map[string]interface{}{
		"method": "core.get_session_status",
		"params": []interface{}{
			[]string{
				"download_rate", "upload_rate", "total_download", "total_upload",
				"free_space", "dht", "lsd", "pex",
			},
		},
		"id": 4,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	// Get torrent count
	countPayload := map[string]interface{}{
		"method": "core.get_torrents_status",
		"params": []interface{}{
			map[string]interface{}{},
			[]string{"state"},
		},
		"id": 5,
	}

	jsonData, err = json.Marshal(countPayload)
	if err != nil {
		return nil, err
	}

	req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var countResponse map[string]interface{}
	err = json.Unmarshal(body, &countResponse)
	if err != nil {
		return nil, err
	}

	// Get daemon info
	infoPayload := map[string]interface{}{
		"method": "daemon.info",
		"params": []interface{}{},
		"id": 6,
	}

	jsonData, err = json.Marshal(infoPayload)
	if err != nil {
		return nil, err
	}

	req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var infoResponse map[string]interface{}
	err = json.Unmarshal(body, &infoResponse)
	if err != nil {
		return nil, err
	}

	// Extract values from responses with nil checks
	result, ok := response["result"].(map[string]interface{})
	if !ok || result == nil {
		return nil, fmt.Errorf("invalid response format: missing or invalid result field")
	}

	torrentsResult, ok := countResponse["result"].(map[string]interface{})
	if !ok || torrentsResult == nil {
		return nil, fmt.Errorf("invalid torrent count response format: missing or invalid result field")
	}

	// Handle potentially nil infoResult
	var infoResult map[string]interface{}
	infoResultVal, ok := infoResponse["result"]
	if ok && infoResultVal != nil {
		infoResult, ok = infoResultVal.(map[string]interface{})
		if !ok {
			infoResult = map[string]interface{}{}
		}
	} else {
		infoResult = map[string]interface{}{}
	}

	// Count active torrents with safe type assertions
	activeTorrents := 0
	for _, data := range torrentsResult {
		torrentData, ok := data.(map[string]interface{})
		if !ok || torrentData == nil {
			continue
		}

		stateVal, ok := torrentData["state"]
		if !ok || stateVal == nil {
			continue
		}

		state, ok := stateVal.(string)
		if !ok {
			continue
		}

		if state == "Downloading" || state == "Seeding" {
			activeTorrents++
		}
	}

	// Extract values with type assertions and safe default values
	var downloadRate, uploadRate, totalDownload, totalUpload, freeSpace float64
	var dht, lsd, pex bool

	if val, ok := result["download_rate"]; ok && val != nil {
		downloadRate, _ = val.(float64)
	}
	if val, ok := result["upload_rate"]; ok && val != nil {
		uploadRate, _ = val.(float64)
	}
	if val, ok := result["total_download"]; ok && val != nil {
		totalDownload, _ = val.(float64)
	}
	if val, ok := result["total_upload"]; ok && val != nil {
		totalUpload, _ = val.(float64)
	}
	if val, ok := result["free_space"]; ok && val != nil {
		freeSpace, _ = val.(float64)
	}
	if val, ok := result["dht"]; ok && val != nil {
		dht, _ = val.(bool)
	}
	if val, ok := result["lsd"]; ok && val != nil {
		lsd, _ = val.(bool)
	}
	if val, ok := result["pex"]; ok && val != nil {
		pex, _ = val.(bool)
	}

	// Extract version and libtorrent with safe defaults
	version := "Unknown"
	if versionVal, ok := infoResult["version"]; ok && versionVal != nil {
		if versionStr, ok := versionVal.(string); ok {
			version = versionStr
		}
	}

	libtorrent := "Unknown"
	if libtorrentVal, ok := infoResult["libtorrent"]; ok && libtorrentVal != nil {
		if libtorrentStr, ok := libtorrentVal.(string); ok {
			libtorrent = libtorrentStr
		}
	}

	status := &models.DelugeServerStatus{
		ServerID:       server.ID,
		Name:           server.Name,
		Connected:      true,
		DownloadRate:   int64(downloadRate),
		UploadRate:     int64(uploadRate),
		TotalDownload:  int64(totalDownload),
		TotalUpload:    int64(totalUpload),
		FreeSpace:      int64(freeSpace),
		ActiveTorrents: activeTorrents,
		TotalTorrents:  len(torrentsResult),
		DHT:            dht,
		LSD:            lsd,
		PEX:            pex,
		Version:        version,
		Libtorrent:     libtorrent,
	}

	return status, nil
}

// authenticateWithServer authenticates with the Deluge server and returns the session cookie
func (h *DelugeHandler) authenticateWithServer(server *models.DelugeServer) (*http.Cookie, error) {
	// Authenticate with the server
	authPayload := map[string]interface{}{
		"method": "auth.login",
		"params": []string{server.Password},
		"id":     1,
	}

	jsonData, err := json.Marshal(authPayload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var authResponse map[string]interface{}
	err = json.Unmarshal(body, &authResponse)
	if err != nil {
		return nil, err
	}

	// Check if authentication was successful
	result, ok := authResponse["result"].(bool)
	if !ok || !result {
		return nil, fmt.Errorf("authentication failed: %v", authResponse["error"])
	}

	// Get session cookie
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "_session_id" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		return nil, fmt.Errorf("no session cookie found")
	}

	return sessionCookie, nil
}

// addTorrentToServer adds a torrent to the Deluge server
func (h *DelugeHandler) addTorrentToServer(server *models.DelugeServer, torrentURL string, downloadLocation string) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Add torrent
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	options := map[string]interface{}{}
	if downloadLocation != "" {
		options["download_location"] = downloadLocation
	}

	payload := map[string]interface{}{
		"method": "core.add_torrent_url",
		"params": []interface{}{
			torrentURL,
			options,
		},
		"id": 7,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}

// pauseTorrentOnServer pauses a torrent on the Deluge server
func (h *DelugeHandler) pauseTorrentOnServer(server *models.DelugeServer, torrentID string) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Pause torrent
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	payload := map[string]interface{}{
		"method": "core.pause_torrent",
		"params": []interface{}{
			[]string{torrentID},
		},
		"id": 8,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}

// resumeTorrentOnServer resumes a torrent on the Deluge server
func (h *DelugeHandler) resumeTorrentOnServer(server *models.DelugeServer, torrentID string) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Resume torrent
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	payload := map[string]interface{}{
		"method": "core.resume_torrent",
		"params": []interface{}{
			[]string{torrentID},
		},
		"id": 9,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}

// removeTorrentFromServer removes a torrent from the Deluge server
func (h *DelugeHandler) removeTorrentFromServer(server *models.DelugeServer, torrentID string, removeData bool) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Remove torrent
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	payload := map[string]interface{}{
		"method": "core.remove_torrent",
		"params": []interface{}{
			torrentID,
			removeData,
		},
		"id": 10,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}

// pauseAllTorrentsOnServer pauses all torrents on the Deluge server
func (h *DelugeHandler) pauseAllTorrentsOnServer(server *models.DelugeServer) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Pause all torrents
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	payload := map[string]interface{}{
		"method": "core.pause_all_torrents",
		"params": []interface{}{},
		"id": 11,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}

// resumeAllTorrentsOnServer resumes all torrents on the Deluge server
func (h *DelugeHandler) resumeAllTorrentsOnServer(server *models.DelugeServer) error {
	// Authenticate with the server
	sessionCookie, err := h.authenticateWithServer(server)
	if err != nil {
		return err
	}

	// Resume all torrents
	url := fmt.Sprintf("%s://%s:%d/json", server.Protocol, server.Host, server.Port)
	client := &http.Client{}

	payload := map[string]interface{}{
		"method": "core.resume_all_torrents",
		"params": []interface{}{},
		"id": 12,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.AddCookie(sessionCookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	// Check if the API call was successful
	if response["error"] != nil {
		return fmt.Errorf("API call failed: %v", response["error"])
	}

	return nil
}
