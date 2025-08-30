package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
	"webui-skeleton/internal/repository"

	"github.com/gin-gonic/gin"
)

type PlexHandler struct {
	config  *config.Config
	db      *database.DB
	authSvc *auth.Service
	repo    repository.PlexServerRepositoryInterface
}

// NewPlexHandler creates a new Plex handler
func NewPlexHandler(config *config.Config, db *database.DB, authSvc *auth.Service, repo repository.PlexServerRepositoryInterface) *PlexHandler {
	return &PlexHandler{
		config:  config,
		db:      db,
		authSvc: authSvc,
		repo:    repo,
	}
}

// Unified handler: Serve Plex libraries as JSON or HTMX HTML
func (h *PlexHandler) GetPlexLibraries(c *gin.Context) {
	logger.Log.Debug().Msg("[GetPlexLibraries] Handler invoked")
	userObj, userExists := c.Get("user_obj")
	if !userExists {
		logger.Log.Error().Msg("[GetPlexLibraries] No user object found")
		c.JSON(http.StatusBadGateway, gin.H{"error": "No user object found"})
		return
	}
	user, ok := userObj.(*models.User)
	if !ok {
		logger.Log.Error().Msg("[GetPlexLibraries] Invalid user object")
		c.JSON(http.StatusBadGateway, gin.H{"error": "Invalid user object"})
		return
	}
	dto, err := h.fetchPlexLibraries(c, user)
	if err != nil {
		logger.Log.Error().Msgf("[GetPlexLibraries] Failed to fetch Plex libraries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Plex libraries"})
		return
	}
	logger.Log.Trace().Msgf("[GetPlexLibraries] Parsed libraries DTO: %+v", dto)
	if c.GetHeader("HX-Request") != "" {
		c.HTML(http.StatusOK, "plex_libraries.html", gin.H{
			"libraries": dto.MediaContainer.Directory,
		})
	} else {
		c.JSON(http.StatusOK, dto)
	}
}

func (h *PlexHandler) RenderPlexRecentlyAddedHTMX(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)

	workingURL, err := h.getWorkingURLServer(user, c.ClientIP())
	if err != nil || workingURL == "" {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] No working connection for user %s", user.PlexUsername)
		c.String(http.StatusBadGateway, "No working Plex server connection found")
		return
	}

	// Use the correct endpoint for recently added items
	recentlyAddedURL := workingURL + "/library/recentlyAdded"
	client := &http.Client{}
	req, err := http.NewRequest("GET", recentlyAddedURL, nil)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Failed to create request for user %s: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to create request")
		return
	}
	req.Header.Set("X-Plex-Token", user.PlexToken)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Failed to contact Plex server for user %s: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to contact Plex server")
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log.Error().Msgf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Plex server error for user %s. Status: %d", user.PlexUsername, resp.StatusCode)
		c.String(resp.StatusCode, "Plex server error")
		return
	}

	// Debug: log raw JSON response
	var rawBody []byte
	rawBody, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Failed to read Plex response body for user %s: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to read Plex response body")
		return
	}

	// Parse JSON response into DTO
	var dto models.PlexRecentlyAddedDTO
	if err := json.Unmarshal(rawBody, &dto); err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Failed to parse Plex JSON response for user %s: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to parse Plex JSON response")
		return
	}

	logger.Log.Trace().Msgf("[HTMX][RenderPlexRecentlyAddedHTMX] Parsed recently added for user %s: %+v", user.PlexUsername, dto.MediaContainer.Metadata)

	// Group and sort Metadata by LibrarySectionTitle
	grouped := make(map[string][]models.PlexMetadata)
	for _, item := range dto.MediaContainer.Metadata {
		lib := item.LibrarySectionTitle
		grouped[lib] = append(grouped[lib], item)
	}
	// Sort each group by AddedAt descending
	for lib := range grouped {
		items := grouped[lib]
		// Simple bubble sort for demonstration; use sort.Slice in real code
		for i := 0; i < len(items)-1; i++ {
			for j := 0; j < len(items)-i-1; j++ {
				if items[j].AddedAt < items[j+1].AddedAt {
					items[j], items[j+1] = items[j+1], items[j]
				}
			}
		}
		grouped[lib] = items
	}

	if c.GetHeader("HX-Request") != "" {
		c.HTML(http.StatusOK, "plex_recently_added.html", gin.H{
			"GroupedRecentlyAdded": grouped,
			"PlexBaseURL":          workingURL,
			"PlexToken":            user.PlexToken,
		})
	} else {
		c.JSON(http.StatusOK, dto)
	}
}

// HTMX handler: Render Plex servers list and allow setting preferred server
func (h *PlexHandler) RenderPlexServersHTMX(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	plexToken := user.PlexToken
	clientID := user.PlexID

	logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Called for user: %s, Token: %s, ClientID: %s", user.PlexUsername, plexToken, clientID)
	if plexToken == "" || clientID == "" {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Missing Plex token or client identifier for user: %s", user.PlexUsername)
		c.String(http.StatusBadRequest, "Missing Plex token or client identifier")
		return
	}

	plexURL := "https://plex.tv/api/v2/resources"
	client := &http.Client{}
	logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Requesting Plex servers from %s for user: %s", plexURL, user.PlexUsername)
	logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Preparing HTTP request: url=%s, clientID=%s, token=%s", plexURL, clientID, plexToken)
	req, err := http.NewRequest("GET", plexURL, nil)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Failed to create request for user: %s, error: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to create request")
		return
	}
	logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Setting headers: X-Plex-Client-Identifier=%s, X-Plex-Token=%s", clientID, plexToken)
	req.Header.Set("X-Plex-Client-Identifier", clientID)
	req.Header.Set("X-Plex-Token", plexToken)
	req.Header.Set("Accept", "application/json")

	logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Executing HTTP request for user: %s", user.PlexUsername)
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] HTTP request failed for user: %s, error: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to fetch Plex servers")
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Log.Error().Msgf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Failed to fetch Plex servers for user %s. Status: %d", user.PlexUsername, resp.StatusCode)
		c.String(resp.StatusCode, "Failed to fetch Plex servers")
		return
	}

	var rawServers []models.PlexServer
	if err := json.NewDecoder(resp.Body).Decode(&rawServers); err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Failed to unmarshal Plex servers JSON for user %s: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to parse Plex servers JSON")
		return
	}
	servers := []models.PlexServer{}
	// Upsert servers into the database
	servers, err = h.repo.BatchUpsertAndFetchServers(user, rawServers)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Failed to upsert servers for user %s: %v", user.PlexUsername, err)
	}

	logger.Log.Trace().Msgf("[HTMX][RenderPlexServersHTMX] Parsed and upserted servers for user %s: %+v", user.PlexUsername, servers)

	preferredServer, err := h.repo.GetPreferredPlexServer(user.ID)
	var preferredServerID = -1
	if err == nil && preferredServer != nil {
		preferredServerID = preferredServer.ID
		logger.Log.Debug().Msgf("[HTMX][RenderPlexServersHTMX] Preferred server for user %s: %d", user.PlexUsername, preferredServerID)
	} else if err != nil {
		logger.Log.Error().Msgf("[HTMX][RenderPlexServersHTMX] Failed to get preferred server for user %s: %v", user.PlexUsername, err)
	}

	c.HTML(http.StatusOK, "plex_servers.html", gin.H{
		"servers":            servers,
		"preferredServer":    preferredServerID,
		"hasPreferredServer": preferredServerID != -1,
	})
}

// HTMX handler: Set preferred Plex server
func (h *PlexHandler) SetPreferredPlexServer(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	serverIDStr := c.PostForm("server_id")

	logger.Log.Debug().Msgf("[HTMX][SetPreferredPlexServer] Called for user: %s, serverID: %s", user.PlexUsername, serverIDStr)

	if serverIDStr == "" {
		logger.Log.Error().Msgf("[HTMX][SetPreferredPlexServer] Missing server ID for user: %s", user.PlexUsername)
		c.String(http.StatusBadRequest, "Missing server ID")
		return
	}

	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][SetPreferredPlexServer] Invalid server ID for user: %s, error: %v", user.PlexUsername, err)
		c.String(http.StatusBadRequest, "Invalid server ID")
		return
	}

	// Update preferred server in plex_servers table
	err = h.repo.SetPreferredPlexServer(user.ID, serverID)
	if err != nil {
		logger.Log.Error().Msgf("[HTMX][SetPreferredPlexServer] Failed to set preferred server for user: %s, error: %v", user.PlexUsername, err)
		c.String(http.StatusInternalServerError, "Failed to set preferred server in plex_servers")
		return
	}

	logger.Log.Debug().Msgf("[HTMX][SetPreferredPlexServer] Preferred server updated for user: %s, serverID: %d", user.PlexUsername, serverID)
	c.String(http.StatusOK, "Preferred server updated")
}

// Refresh a Plex library section
func (h *PlexHandler) RefreshPlexLibrary(c *gin.Context) {
	userObj, _ := c.Get("user_obj")
	user, _ := userObj.(*models.User)
	plexToken := user.PlexToken
	sectionID := c.Param("id")

	// Use preferred server logic
	preferredServer, err := h.repo.GetPreferredPlexServer(user.ID)
	if err != nil || preferredServer == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "No working Plex server connection found"})
		return
	}

	var refreshURL string
	for _, conn := range preferredServer.Connections {
		tryURL := conn.URI + "/library/sections/" + sectionID + "/refresh"
		client := &http.Client{}
		req, err := http.NewRequest("GET", tryURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("X-Plex-Token", plexToken)
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			refreshURL = tryURL
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if refreshURL == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "No working Plex server connection found for refresh"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Library refresh triggered"})
}

// Handler: Render Plex Configuration Page
func (h *PlexHandler) RenderPlexConfigurePage(c *gin.Context) {
	userObj, _ := c.Get("user_obj")

	c.HTML(http.StatusOK, "plex_configure.html", gin.H{
		"user": userObj,
	})
}

// fetchPlexLibraries fetches and parses Plex libraries for a user
func (h *PlexHandler) fetchPlexLibraries(c *gin.Context, user *models.User) (*models.PlexLibrariesDTO, error) {
	plexToken := user.PlexToken
	workingURI, err := h.getWorkingURLServer(user, c.ClientIP())
	if err != nil || workingURI == "" {
		return nil, err
	}
	plexURL := workingURI + "/library/sections"
	client := &http.Client{}
	req, err := http.NewRequest("GET", plexURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", plexToken)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	var dto models.PlexLibrariesDTO
	if err := json.NewDecoder(resp.Body).Decode(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

// Returns the first working Plex server connection URL for the user
func (h *PlexHandler) getWorkingURLServer(user *models.User, clientIP string) (string, error) {
	logger.Log.Debug().Msgf("[getWorkingURLServer] Called for user: %s (ID: %d)", user.PlexUsername, user.ID)
	preferredServer, err := h.repo.GetPreferredPlexServer(user.ID)
	if err != nil {
		logger.Log.Error().Msgf("[getWorkingURLServer] Error getting preferred server for user %s: %v", user.PlexUsername, err)
		return "", err
	}
	if preferredServer == nil {
		logger.Log.Error().Msgf("[getWorkingURLServer] No preferred server found for user %s", user.PlexUsername)
		return "", nil
	}
	logger.Log.Trace().Msgf("[getWorkingURLServer] preferredServer: %+v\n", preferredServer)
	logger.Log.Trace().Msgf("[getWorkingURLServer] preferredServer.Connections: %+v\n", preferredServer.Connections)

	logger.Log.Debug().Msgf("[getWorkingURLServer] clientIP: %s", clientIP)
	for _, conn := range preferredServer.Connections {
		logger.Log.Debug().Msgf("[getWorkingURLServer] Checking connection URI: %s, Local: %v", conn.URI, conn.Local)
	}

	isSameNetwork := func(clientIP, connURI string) bool {
		uri := strings.TrimPrefix(connURI, "http://")
		uri = strings.TrimPrefix(uri, "https://")
		parts := strings.Split(uri, ":")
		host := parts[0]

		logger.Log.Debug().Msgf("[getWorkingURLServer] Comparing clientIP: %s with host: %s", clientIP, host)

		// Handle IPv6 localhost
		if clientIP == "::1" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
			logger.Log.Debug().Msg("[getWorkingURLServer] Matched IPv6 localhost")
			return true
		}
		// Handle IPv4 localhost
		if clientIP == "127.0.0.1" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
			logger.Log.Debug().Msg("[getWorkingURLServer] Matched IPv4 localhost")
			return true
		}
		// Handle LAN IPs
		isLAN := func(ip string) bool {
			return strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") ||
				(strings.HasPrefix(ip, "172.") && func() bool {
					parts := strings.Split(ip, ".")
					if len(parts) < 2 {
						return false
					}
					sec, _ := strconv.Atoi(parts[1])
					return sec >= 16 && sec <= 31
				}())
		}
		if isLAN(clientIP) && isLAN(host) {
			logger.Log.Debug().Msg("[getWorkingURLServer] Matched LAN IPs")
			return true
		}
		// Fallback: compare first 2 octets for legacy behavior
		clientParts := strings.Split(clientIP, ".")
		hostParts := strings.Split(host, ".")
		if len(clientParts) >= 2 && len(hostParts) >= 2 && clientParts[0] == hostParts[0] && clientParts[1] == hostParts[1] {
			logger.Log.Debug().Msg("[getWorkingURLServer] Matched first 2 octets")
			return true
		}
		logger.Log.Debug().Msg("[getWorkingURLServer] No network match")
		return false
	}

	var localConn, lanConn, externalConn string
	for _, conn := range preferredServer.Connections {
		if isSameNetwork(clientIP, conn.URI) {
			logger.Log.Debug().Msgf("[getWorkingURLServer] Found local connection: %s", conn.URI)
			localConn = conn.URI
			break // Prefer first matching local connection
		}
		// Prefer LAN connection if client is localhost and connection is LAN
		uri := strings.TrimPrefix(conn.URI, "http://")
		uri = strings.TrimPrefix(uri, "https://")
		parts := strings.Split(uri, ":")
		host := parts[0]
		isLAN := func(ip string) bool {
			return strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") ||
				(strings.HasPrefix(ip, "172.") && func() bool {
					parts := strings.Split(ip, ".")
					if len(parts) < 2 {
						return false
					}
					sec, _ := strconv.Atoi(parts[1])
					return sec >= 16 && sec <= 31
				}())
		}
		if (clientIP == "127.0.0.1" || clientIP == "::1") && isLAN(host) && lanConn == "" {
			logger.Log.Debug().Msgf("[getWorkingURLServer] Found LAN connection candidate for localhost client: %s", conn.URI)
			lanConn = conn.URI
		}
		if !conn.Local && externalConn == "" {
			logger.Log.Debug().Msgf("[getWorkingURLServer] Found external connection candidate: %s", conn.URI)
			externalConn = conn.URI // First external connection
		}
	}

	if localConn != "" {
		logger.Log.Debug().Msgf("[getWorkingURLServer] Selected local connection: %s", localConn)
		return localConn, nil
	}
	if lanConn != "" {
		logger.Log.Debug().Msgf("[getWorkingURLServer] Selected LAN connection for localhost client: %s", lanConn)
		return lanConn, nil
	}
	if externalConn != "" {
		logger.Log.Debug().Msgf("[getWorkingURLServer] Selected external connection: %s", externalConn)
		return externalConn, nil
	}
	logger.Log.Error().Msgf("[getWorkingURLServer] No working connection found for user %s", user.PlexUsername)
	return "", nil
}
