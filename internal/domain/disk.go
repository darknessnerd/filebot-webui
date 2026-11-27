package domain

// DiskUsage describes space on one mounted filesystem.
type DiskUsage struct {
	Label       string
	Path        string
	Total       int64
	Free        int64
	Used        int64
	UsedPercent int
}
