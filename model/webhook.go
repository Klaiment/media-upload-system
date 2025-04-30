package model

// RadarrWebhook représente le payload envoyé par Radarr
type RadarrWebhook struct {
	Movie              *Movie              `json:"movie,omitempty"`
	RemoteMovie        *RemoteMovie        `json:"remoteMovie,omitempty"`
	MovieFile          *MovieFile          `json:"movieFile,omitempty"`
	Series             *Series             `json:"series,omitempty"`
	Episodes           []*Episode          `json:"episodes,omitempty"`
	IsUpgrade          bool                `json:"isUpgrade"`
	DownloadClient     string              `json:"downloadClient"`
	DownloadClientType string              `json:"downloadClientType"`
	DownloadId         string              `json:"downloadId"`
	CustomFormatInfo   *CustomFormatInfo   `json:"customFormatInfo,omitempty"`
	Release            *Release            `json:"release,omitempty"`
	EventType          string              `json:"eventType"`
	InstanceName       string              `json:"instanceName"`
	ApplicationUrl     string              `json:"applicationUrl"`
}

// Movie représente les informations d'un film
type Movie struct {
	ID               int              `json:"id"`
	Title            string           `json:"title"`
	Year             int              `json:"year"`
	ReleaseDate      string           `json:"releaseDate,omitempty"`
	FolderPath       string           `json:"folderPath"`
	TmdbId           int              `json:"tmdbId"`
	ImdbId           string           `json:"imdbId,omitempty"`
	Overview         string           `json:"overview,omitempty"`
	Genres           []string         `json:"genres,omitempty"`
	Images           []Image          `json:"images,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	OriginalLanguage *OriginalLanguage `json:"originalLanguage,omitempty"`
}

// RemoteMovie représente les informations d'un film distant
type RemoteMovie struct {
	TmdbId  int    `json:"tmdbId"`
	ImdbId  string `json:"imdbId,omitempty"`
	Title   string `json:"title"`
	Year    int    `json:"year"`
}

// MovieFile représente les informations d'un fichier de film
type MovieFile struct {
	ID             int        `json:"id"`
	RelativePath   string     `json:"relativePath"`
	Path           string     `json:"path"`
	Quality        string     `json:"quality"`
	QualityVersion int        `json:"qualityVersion"`
	SceneName      string     `json:"sceneName,omitempty"`
	IndexerFlags   string     `json:"indexerFlags,omitempty"`
	Size           int64      `json:"size"`
	DateAdded      string     `json:"dateAdded"`
	Languages      []Language `json:"languages,omitempty"`
	MediaInfo      *MediaInfo `json:"mediaInfo,omitempty"`
	SourcePath     string     `json:"sourcePath,omitempty"`
}

// Image représente une image associée à un média
type Image struct {
	CoverType string `json:"coverType"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
}

// OriginalLanguage représente la langue originale d'un média
type OriginalLanguage struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Language représente une langue
type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MediaInfo représente les informations techniques d'un média
type MediaInfo struct {
	AudioChannels         float64  `json:"audioChannels"`
	AudioCodec            string   `json:"audioCodec"`
	AudioLanguages        []string `json:"audioLanguages"`
	Height                int      `json:"height"`
	Width                 int      `json:"width"`
	Subtitles             []string `json:"subtitles,omitempty"`
	VideoCodec            string   `json:"videoCodec"`
	VideoDynamicRange     string   `json:"videoDynamicRange"`
	VideoDynamicRangeType string   `json:"videoDynamicRangeType"`
}

// CustomFormatInfo représente les informations de format personnalisé
type CustomFormatInfo struct {
	CustomFormats     []CustomFormat `json:"customFormats"`
	CustomFormatScore int            `json:"customFormatScore"`
}

// CustomFormat représente un format personnalisé
type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Release représente les informations de release
type Release struct {
	ReleaseTitle string `json:"releaseTitle"`
	Indexer      string `json:"indexer"`
	Size         int64  `json:"size"`
}

// Series représente les informations d'une série
type Series struct {
	ID       int       `json:"id"`
	Title    string    `json:"title"`
	Year     int       `json:"year"`
	Path     string    `json:"path"`
	TvdbId   int       `json:"tvdbId"`
	TmdbId   int       `json:"tmdbId,omitempty"`
	ImdbId   string    `json:"imdbId,omitempty"`
	Seasons  []*Season `json:"seasons,omitempty"`
}

// Season représente une saison d'une série
type Season struct {
	SeasonNumber int `json:"seasonNumber"`
}

// Episode représente un épisode d'une série
type Episode struct {
	ID            int    `json:"id"`
	EpisodeNumber int    `json:"episodeNumber"`
	SeasonNumber  int    `json:"seasonNumber"`
	Title         string `json:"title"`
	AirDate       string `json:"airDate,omitempty"`
}