package domain

type FileBotJob struct {
	TorrentIDs  []string
	SourcePaths []string
	DB          string
	Action      string
	Conflict    string
	LogLevel    string
	Format      string
	Filter      string
	Query       string
	Recursive   bool
	Output      string
}

type FileBotResult struct {
	Successes []string
	Errors    []string
	RawOutput string
}

type MovieMatch struct {
	ID    int
	Title string
	Year  int
}

type TVMatch struct {
	ID   int
	Name string
	Year int
}

type AnimeMatch struct {
	ID    int
	Title string
	Year  int
}
