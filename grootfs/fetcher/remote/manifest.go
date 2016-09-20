package remote

type Manifest struct {
	SchemaVersion   int
	Layers          []Layer
	ConfigCacheKey  string
	V1Compatibility []string
}

type Layer struct {
	BlobID string
	Size   int64
}

type SchemaV1Manifest struct {
	FSLayers []map[string]string `json:"fsLayers"`
	History  []History           `json:"history"`
}

type History struct {
	V1Compatibility string `json:"v1Compatibility"`
}

type V1Compatibility struct {
	ID string `json:"id"`
}
