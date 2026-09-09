package domain

type Action string

const (
	ActionMove     Action = "move"
	ActionCopy     Action = "copy"
	ActionSymlink  Action = "symlink"
	ActionHardlink Action = "hardlink"
	ActionTest     Action = "test"
)

type FileBotJob struct {
	TorrentIDs  []string
	SourcePaths []string
	DB          string
	Action      Action
	Conflict    string
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
