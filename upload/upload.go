package upload

// Uploader définit l'interface pour les services d'upload
type Uploader interface {
	Name() string
	IsEnabled() bool
	UploadFile(filePath, title string) (*UploadResult, error)
}

// UploadResult représente le résultat d'un upload
type UploadResult struct {
	Success  bool
	Hoster   string
	FileCode string
	URL      string
	Embed    string
}
