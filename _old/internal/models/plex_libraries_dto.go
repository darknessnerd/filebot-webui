package models

// PlexLibraryLocation represents a location for a Plex library section
type PlexLibraryLocation struct {
	ID   int    `json:"id" xml:"id,attr"`
	Path string `json:"path" xml:"path,attr"`
}

// PlexLibraryDirectory represents a Plex library section
type PlexLibraryDirectory struct {
	AllowSync        bool                  `json:"allowSync" xml:"allowSync,attr"`
	Art              string                `json:"art" xml:"art,attr"`
	Composite        string                `json:"composite" xml:"composite,attr"`
	Filters          bool                  `json:"filters" xml:"filters,attr"`
	Refreshing       bool                  `json:"refreshing" xml:"refreshing,attr"`
	Thumb            string                `json:"thumb" xml:"thumb,attr"`
	Key              string                `json:"key" xml:"key,attr"`
	Type             string                `json:"type" xml:"type,attr"`
	Title            string                `json:"title" xml:"title,attr"`
	Agent            string                `json:"agent" xml:"agent,attr"`
	Scanner          string                `json:"scanner" xml:"scanner,attr"`
	Language         string                `json:"language" xml:"language,attr"`
	UUID             string                `json:"uuid" xml:"uuid,attr"`
	UpdatedAt        int64                 `json:"updatedAt" xml:"updatedAt,attr"`
	CreatedAt        int64                 `json:"createdAt" xml:"createdAt,attr"`
	ScannedAt        int64                 `json:"scannedAt" xml:"scannedAt,attr"`
	Content          bool                  `json:"content" xml:"content,attr"`
	Directory        bool                  `json:"directory" xml:"directory,attr"`
	ContentChangedAt int64                 `json:"contentChangedAt" xml:"contentChangedAt,attr"`
	Hidden           int                   `json:"hidden" xml:"hidden,attr"`
	Location         []PlexLibraryLocation `json:"Location" xml:"Location"`
}

// PlexLibraryMediaContainer represents the top-level container for Plex libraries
type PlexLibraryMediaContainer struct {
	Size      int                    `json:"size" xml:"size,attr"`
	AllowSync bool                   `json:"allowSync" xml:"allowSync,attr"`
	Title1    string                 `json:"title1" xml:"title1,attr"`
	Directory []PlexLibraryDirectory `json:"Directory" xml:"Directory"`
}

// PlexLibrariesDTO is the root DTO for the Plex libraries response
type PlexLibrariesDTO struct {
	MediaContainer PlexLibraryMediaContainer `json:"MediaContainer" xml:"MediaContainer"`
}
