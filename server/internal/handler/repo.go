package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

type TestRepoRequest struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

type TestRepoResponse struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

func (h *Handler) TestRepo(w http.ResponseWriter, r *http.Request) {
	var req TestRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "git") {
		writeError(w, http.StatusBadRequest, "url must start with http, https, or git")
		return
	}

	authURL := injectRepoToken(req.URL, req.Token)

	slog.Info("testing repo connectivity", "url", req.URL)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", authURL, "HEAD")
	output, err := cmd.CombinedOutput()

	if err != nil {
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = err.Error()
		}
		// Redact any token from error output
		if req.Token != "" {
			errMsg = strings.ReplaceAll(errMsg, req.Token, "***")
		}
		writeJSON(w, http.StatusOK, TestRepoResponse{
			OK:    false,
			Error: errMsg,
		})
		return
	}

	defaultBranch := parseDefaultBranch(string(output))

	writeJSON(w, http.StatusOK, TestRepoResponse{
		OK:            true,
		DefaultBranch: defaultBranch,
	})
}

// injectRepoToken returns a URL with credentials embedded for private repo access.
// For HTTPS URLs it sets the userinfo to "oauth2:<token>". SSH URLs (git@...)
// are returned unchanged. If token is empty the original URL is returned as-is.
func injectRepoToken(rawURL, token string) string {
	if token == "" {
		return rawURL
	}
	if !strings.HasPrefix(rawURL, "https://") && !strings.HasPrefix(rawURL, "http://") {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	parsed.User = url.UserPassword("oauth2", token)
	return parsed.String()
}

// parseDefaultBranch extracts the default branch name from git ls-remote --symref output.
// It looks for a line like "ref: refs/heads/main\tHEAD".
func parseDefaultBranch(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "ref: refs/heads/") && strings.HasSuffix(line, "\tHEAD") {
			ref := strings.TrimPrefix(line, "ref: refs/heads/")
			ref = strings.TrimSuffix(ref, "\tHEAD")
			return ref
		}
	}
	return ""
}

