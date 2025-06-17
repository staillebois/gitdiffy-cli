package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gitdiffy/model"

	"github.com/spf13/viper"
)

func GenerateCommitMessages(diff string) ([]model.Commit, error) {
	req := model.GenerateMessageRequest{LicenseKey: viper.GetString("license"), Diff: diff}
	body, _ := json.Marshal(req)

	resp, err := http.Post("http://localhost:8080/generate-message", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var res model.GenerateMessageResponse
	json.Unmarshal(data, &res)

	if res.Error != "" {
		return nil, fmt.Errorf("API error: %s", res.Error)
	}
	return res.Commits.Commits, nil
}
