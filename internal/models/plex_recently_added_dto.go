// DTO for Plex Recently Added response
package models

// MediaContainer struct for the root object
// Only relevant fields are included for brevity; add more as needed

type PlexRecentlyAddedDTO struct {
	MediaContainer PlexMediaContainer `json:"MediaContainer"`
}

type PlexMediaContainer struct {
	Size       int            `json:"size"`
	Offset     int            `json:"offset"`
	TotalSize  int            `json:"totalSize"`
	Identifier string         `json:"identifier"`
	AllowSync  bool           `json:"allowSync"`
	Meta       PlexMeta       `json:"Meta"`
	Metadata   []PlexMetadata `json:"Metadata"`
}

type PlexMeta struct {
	Type      []PlexMetaType  `json:"Type"`
	FieldType []PlexFieldType `json:"FieldType"`
}

type PlexMetaType struct {
	Key     string       `json:"key"`
	Type    string       `json:"type"`
	Subtype string       `json:"subtype"`
	Title   string       `json:"title"`
	Active  bool         `json:"active"`
	Filter  []PlexFilter `json:"Filter"`
	Sort    []PlexSort   `json:"Sort"`
	Field   []PlexField  `json:"Field"`
}

type PlexFilter struct {
	Filter     string `json:"filter"`
	FilterType string `json:"filterType"`
	Key        string `json:"key"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Advanced   bool   `json:"advanced"`
}

type PlexSort struct {
	Default           string `json:"default"`
	Active            bool   `json:"active"`
	ActiveDirection   string `json:"activeDirection"`
	DefaultDirection  string `json:"defaultDirection"`
	DescKey           string `json:"descKey"`
	FirstCharacterKey string `json:"firstCharacterKey"`
	Key               string `json:"key"`
	Title             string `json:"title"`
}

type PlexField struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	SubType string `json:"subType"`
}

type PlexFieldType struct {
	Type     string         `json:"type"`
	Operator []PlexOperator `json:"Operator"`
}

type PlexOperator struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// Metadata and nested types

type PlexMetadata struct {
	RatingKey                     string              `json:"ratingKey"`
	Key                           string              `json:"key"`
	Guid                          string              `json:"guid"`
	Studio                        string              `json:"studio"`
	SkipChildren                  bool                `json:"skipChildren"`
	LibrarySectionID              int                 `json:"librarySectionID"`
	LibrarySectionTitle           string              `json:"librarySectionTitle"`
	LibrarySectionKey             string              `json:"librarySectionKey"`
	Type                          string              `json:"type"`
	Title                         string              `json:"title"`
	Slug                          string              `json:"slug"`
	ContentRating                 string              `json:"contentRating"`
	Summary                       string              `json:"summary"`
	Rating                        float64             `json:"rating"`
	AudienceRating                float64             `json:"audienceRating"`
	Year                          int                 `json:"year"`
	SeasonCount                   int                 `json:"seasonCount"`
	Tagline                       string              `json:"tagline"`
	FlattenSeasons                string              `json:"flattenSeasons"`
	EpisodeSort                   string              `json:"episodeSort"`
	EnableCreditsMarkerGeneration string              `json:"enableCreditsMarkerGeneration"`
	ShowOrdering                  string              `json:"showOrdering"`
	Thumb                         string              `json:"thumb"`
	Art                           string              `json:"art"`
	Banner                        string              `json:"banner"`
	Duration                      int64               `json:"duration"`
	OriginallyAvailableAt         string              `json:"originallyAvailableAt"`
	AddedAt                       int64               `json:"addedAt"`
	UpdatedAt                     int64               `json:"updatedAt"`
	AudienceRatingImage           string              `json:"audienceRatingImage"`
	ChapterSource                 string              `json:"chapterSource"`
	PrimaryExtraKey               string              `json:"primaryExtraKey"`
	RatingImage                   string              `json:"ratingImage"`
	GrandparentRatingKey          string              `json:"grandparentRatingKey"`
	GrandparentGuid               string              `json:"grandparentGuid"`
	GrandparentKey                string              `json:"grandparentKey"`
	GrandparentTitle              string              `json:"grandparentTitle"`
	GrandparentThumb              string              `json:"grandparentThumb"`
	ParentSlug                    string              `json:"parentSlug"`
	GrandparentSlug               string              `json:"grandparentSlug"`
	GrandparentArt                string              `json:"grandparentArt"`
	GrandparentTheme              string              `json:"grandparentTheme"`
	Media                         []PlexMedia         `json:"Media"`
	Genre                         []PlexTag           `json:"Genre"`
	Country                       []PlexTag           `json:"Country"`
	Director                      []PlexTag           `json:"Director"`
	Writer                        []PlexTag           `json:"Writer"`
	Collection                    []PlexTag           `json:"Collection"`
	Role                          []PlexRole          `json:"Role"`
	Location                      []PlexLocation      `json:"Location"`
	GuidList                      []PlexGuid          `json:"Guid"`
	UltraBlurColors               PlexUltraBlurColors `json:"UltraBlurColors"`
	RatingList                    []PlexRating        `json:"Rating"`
	ImageList                     []PlexImage         `json:"Image"`
	TitleSort                     string              `json:"titleSort"`
	ViewCount                     int                 `json:"viewCount"`
	LastViewedAt                  int64               `json:"lastViewedAt"`
	OriginalTitle                 string              `json:"originalTitle"`
	ViewOffset                    int64               `json:"viewOffset"`
	SkipCount                     int                 `json:"skipCount"`
	Index                         int                 `json:"index"`
	Theme                         string              `json:"theme"`
	LeafCount                     int                 `json:"leafCount"`
	ViewedLeafCount               int                 `json:"viewedLeafCount"`
	ChildCount                    int                 `json:"childCount"`
	HasPremiumExtras              string              `json:"hasPremiumExtras"`
	HasPremiumPrimaryExtra        string              `json:"hasPremiumPrimaryExtra"`
	ParentRatingKey               string              `json:"parentRatingKey"`
	ParentGuid                    string              `json:"parentGuid"`
	ParentStudio                  string              `json:"parentStudio"`
	ParentKey                     string              `json:"parentKey"`
	ParentTitle                   string              `json:"parentTitle"`
	ParentIndex                   int                 `json:"parentIndex"`
	ParentYear                    int                 `json:"parentYear"`
	ParentThumb                   string              `json:"parentThumb"`
	ParentTheme                   string              `json:"parentTheme"`
}

type PlexMedia struct {
	ID                    int        `json:"id"`
	Duration              int64      `json:"duration"`
	Bitrate               int        `json:"bitrate"`
	Width                 int        `json:"width"`
	Height                int        `json:"height"`
	AspectRatio           float64    `json:"aspectRatio"`
	AudioProfile          string     `json:"audioProfile"`
	AudioChannels         int        `json:"audioChannels"`
	AudioCodec            string     `json:"audioCodec"`
	VideoCodec            string     `json:"videoCodec"`
	VideoResolution       string     `json:"videoResolution"`
	Container             string     `json:"container"`
	VideoFrameRate        string     `json:"videoFrameRate"`
	VideoProfile          string     `json:"videoProfile"`
	HasVoiceActivity      bool       `json:"hasVoiceActivity"`
	OptimizedForStreaming int        `json:"optimizedForStreaming"`
	Has64bitOffsets       bool       `json:"has64bitOffsets"`
	Part                  []PlexPart `json:"Part"`
}

type PlexPart struct {
	ID                    int          `json:"id"`
	Key                   string       `json:"key"`
	Duration              int64        `json:"duration"`
	File                  string       `json:"file"`
	Size                  int64        `json:"size"`
	Container             string       `json:"container"`
	AudioProfile          string       `json:"audioProfile"`
	Has64bitOffsets       bool         `json:"has64bitOffsets"`
	OptimizedForStreaming bool         `json:"optimizedForStreaming"`
	VideoProfile          string       `json:"videoProfile"`
	Indexes               string       `json:"indexes"`
	HasThumbnail          string       `json:"hasThumbnail"`
	Stream                []PlexStream `json:"Stream"`
}

type PlexStream struct {
	ID                   int     `json:"id"`
	StreamType           int     `json:"streamType"`
	Default              bool    `json:"default"`
	Selected             bool    `json:"selected"`
	Codec                string  `json:"codec"`
	Index                int     `json:"index"`
	Bitrate              int     `json:"bitrate"`
	ColorPrimaries       string  `json:"colorPrimaries"`
	ColorRange           string  `json:"colorRange"`
	ColorSpace           string  `json:"colorSpace"`
	ColorTrc             string  `json:"colorTrc"`
	BitDepth             int     `json:"bitDepth"`
	ChromaLocation       string  `json:"chromaLocation"`
	StreamIdentifier     string  `json:"streamIdentifier"`
	ChromaSubsampling    string  `json:"chromaSubsampling"`
	CodedHeight          int     `json:"codedHeight"`
	CodedWidth           int     `json:"codedWidth"`
	FrameRate            float64 `json:"frameRate"`
	HasScalingMatrix     bool    `json:"hasScalingMatrix"`
	HearingImpaired      bool    `json:"hearingImpaired"`
	ClosedCaptions       bool    `json:"closedCaptions"`
	EmbeddedInVideo      string  `json:"embeddedInVideo"`
	Height               int     `json:"height"`
	Level                int     `json:"level"`
	Profile              string  `json:"profile"`
	RefFrames            int     `json:"refFrames"`
	ScanType             string  `json:"scanType"`
	Width                int     `json:"width"`
	DisplayTitle         string  `json:"displayTitle"`
	ExtendedDisplayTitle string  `json:"extendedDisplayTitle"`
	Channels             int     `json:"channels"`
	Language             string  `json:"language"`
	LanguageTag          string  `json:"languageTag"`
	LanguageCode         string  `json:"languageCode"`
	AudioChannelLayout   string  `json:"audioChannelLayout"`
	SamplingRate         int     `json:"samplingRate"`
	Title                string  `json:"title"`
	CanAutoSync          bool    `json:"canAutoSync"`
}

type PlexTag struct {
	Tag string `json:"tag"`
}

type PlexRole struct {
	ID     int    `json:"id"`
	Filter string `json:"filter"`
	Thumb  string `json:"thumb"`
	Tag    string `json:"tag"`
	TagKey string `json:"tagKey"`
	Role   string `json:"role"`
}

type PlexLocation struct {
	Path string `json:"path"`
}

type PlexGuid struct {
	ID string `json:"id"`
}

type PlexUltraBlurColors struct {
	TopLeft     string      `json:"topLeft"`
	TopRight    string      `json:"topRight"`
	BottomRight interface{} `json:"bottomRight"`
	BottomLeft  string      `json:"bottomLeft"`
}

type PlexRating struct {
	Image string  `json:"image"`
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

type PlexImage struct {
	Alt  string `json:"alt"`
	Type string `json:"type"`
	URL  string `json:"url"`
}
