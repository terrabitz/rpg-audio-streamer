package domain

// VideoMetadata describes a remote video before it is imported.
type VideoMetadata struct {
	URL           string  `json:"url"`
	Name          string  `json:"name"`
	Duration      float64 `json:"duration"`
	SizeBytes     *int64  `json:"sizeBytes"`
	SizeEstimated bool    `json:"sizeEstimated"`
}
