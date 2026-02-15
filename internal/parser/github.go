package parser

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/fredericrous/cluster-vision/internal/model"
)

// FetchGitHubFile fetches a raw file from a GitHub repository.
func FetchGitHubFile(src *model.GitHubSource) ([]byte, error) {
	ref := src.Ref
	if ref == "" {
		ref = "main"
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", src.Repo, ref, src.FilePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	token, err := readToken(src.TokenFile)
	if err != nil {
		slog.Warn("failed to read github token, proceeding without auth", "tokenFile", src.TokenFile, "error", err)
	} else if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func readToken(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
