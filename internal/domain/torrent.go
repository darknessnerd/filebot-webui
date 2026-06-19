package domain

import "time"

type Torrent struct {
	ID          string
	Name        string
	State       string
	Progress    float64
	DownloadPath string
	Size        int64
	IsFinished  bool
	CompletedOn time.Time
}
