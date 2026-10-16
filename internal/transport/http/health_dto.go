package httpapi

type HealthResponse struct {
	Status string `json:"status"`
}

type VersionResponse struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}
