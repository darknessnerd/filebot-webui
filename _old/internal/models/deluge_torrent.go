package models

import "time"

// DelugeTorrent represents a torrent in the Deluge system
type DelugeTorrent struct {
	ID             string    `json:"id"`              // Torrent hash
	Name           string    `json:"name"`            // Torrent name
	State          string    `json:"state"`           // Current state (downloading, seeding, paused, etc.)
	Progress       float64   `json:"progress"`        // Download progress (0-100)
	DownloadSpeed  int64     `json:"download_speed"`  // Download speed in bytes/sec
	UploadSpeed    int64     `json:"upload_speed"`    // Upload speed in bytes/sec
	ETA            int64     `json:"eta"`             // Estimated time of arrival in seconds
	Size           int64     `json:"size"`            // Total size in bytes
	Downloaded     int64     `json:"downloaded"`      // Downloaded bytes
	Uploaded       int64     `json:"uploaded"`        // Uploaded bytes
	Ratio          float64   `json:"ratio"`           // Upload/download ratio
	Seeds          int       `json:"seeds"`           // Number of seeds
	Peers          int       `json:"peers"`           // Number of peers
	AddedOn        time.Time `json:"added_on"`        // When the torrent was added
	CompletedOn    time.Time `json:"completed_on"`    // When the torrent completed
	DownloadPath   string    `json:"download_path"`   // Download location
	Label          string    `json:"label"`           // Torrent label/category
	IsFinished     bool      `json:"is_finished"`     // Whether the torrent is finished
	IsAutoManaged  bool      `json:"is_auto_managed"` // Whether the torrent is auto-managed
	IsPrivate      bool      `json:"is_private"`      // Whether the torrent is private
	IsSequential   bool      `json:"is_sequential"`   // Whether sequential download is enabled
	IsSuperSeeding bool      `json:"is_super_seeding"` // Whether super-seeding is enabled
	ServerID       int       `json:"server_id"`       // ID of the Deluge server this torrent belongs to
}

// DelugeTorrentListResponse represents the response from the Deluge API for listing torrents
type DelugeTorrentListResponse struct {
	Result map[string]map[string]interface{} `json:"result"`
	Error  interface{}                       `json:"error"`
	ID     int                               `json:"id"`
}

// DelugeServerStatus represents the status of a Deluge server
type DelugeServerStatus struct {
	ServerID       int     `json:"server_id"`
	Name           string  `json:"name"`
	Connected      bool    `json:"connected"`
	DownloadRate   int64   `json:"download_rate"`   // Download rate in bytes/sec
	UploadRate     int64   `json:"upload_rate"`     // Upload rate in bytes/sec
	TotalDownload  int64   `json:"total_download"`  // Total downloaded bytes in session
	TotalUpload    int64   `json:"total_upload"`    // Total uploaded bytes in session
	FreeSpace      int64   `json:"free_space"`      // Free disk space in bytes
	ActiveTorrents int     `json:"active_torrents"` // Number of active torrents
	TotalTorrents  int     `json:"total_torrents"`  // Total number of torrents
	DHT            bool    `json:"dht"`             // Whether DHT is enabled
	LSD            bool    `json:"lsd"`             // Whether LSD is enabled
	PEX            bool    `json:"pex"`             // Whether PEX is enabled
	Version        string  `json:"version"`         // Deluge version
	Libtorrent     string  `json:"libtorrent"`      // Libtorrent version
	Error          string  `json:"error"`           // Error message if any
}