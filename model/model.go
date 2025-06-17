package model

type GenerateMessageRequest struct {
	LicenseKey string `json:"licenseKey"`
	Diff       string `json:"diff"`
}

type Commit struct {
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

type GenerateMessageResponse struct {
	Commits struct {
		Commits []Commit `json:"commits"`
	} `json:"commits"`
	Error string `json:"error,omitempty"`
}
