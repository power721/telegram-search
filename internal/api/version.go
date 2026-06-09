package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tg-search/internal/build"
	"tg-search/internal/model"
)

var githubLatestReleaseURL = "https://api.github.com/repos/power721/tg-search/releases/latest"
var githubHTTPClient = http.DefaultClient

func (h handlers) getVersionSettings(c *gin.Context) {
	info, err := loadVersionInfo(c.Request.Context(), githubHTTPClient, shouldCheckUpdate(c.Query("check_update")))
	if err != nil {
		errorJSON(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h handlers) getSystemInfoSettings(c *gin.Context) {
	c.JSON(http.StatusOK, loadSystemInfo())
}

func loadSystemInfo() model.SystemInfoResponse {
	hostname, _ := os.Hostname()
	return model.SystemInfoResponse{
		Name:         systemName(runtime.GOOS),
		Version:      systemVersion(runtime.GOOS),
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		CPUCount:     runtime.NumCPU(),
		Hostname:     hostname,
	}
}

func systemName(goos string) string {
	switch goos {
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return goos
	}
}

func systemVersion(goos string) string {
	if goos != "linux" {
		return ""
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func loadVersionInfo(ctx context.Context, client *http.Client, checkUpdate bool) (model.VersionInfoResponse, error) {
	current := strings.TrimSpace(build.Version)
	if current == "" {
		current = "dev"
	}
	if !checkUpdate {
		return model.VersionInfoResponse{CurrentVersion: current}, nil
	}
	latest, err := fetchLatestGitHubRelease(ctx, client)
	if err != nil {
		return model.VersionInfoResponse{CurrentVersion: current}, err
	}
	return model.VersionInfoResponse{
		CurrentVersion:  current,
		LatestVersion:   latest.TagName,
		LatestURL:       latest.HTMLURL,
		UpdateAvailable: newerSemver(latest.TagName, current),
	}, nil
}

func shouldCheckUpdate(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true" || value == "yes"
}

type githubReleaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestGitHubRelease(ctx context.Context, client *http.Client) (githubReleaseResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return githubReleaseResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tg-search")
	resp, err := client.Do(req)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("check GitHub release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubReleaseResponse{}, fmt.Errorf("check GitHub release: status %d", resp.StatusCode)
	}
	var latest githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return githubReleaseResponse{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	if strings.TrimSpace(latest.TagName) == "" {
		return githubReleaseResponse{}, fmt.Errorf("GitHub release tag_name is empty")
	}
	return latest, nil
}

func newerSemver(latest string, current string) bool {
	latestParts, ok := parseSemverParts(latest)
	if !ok {
		return false
	}
	currentParts, ok := parseSemverParts(current)
	if !ok {
		return false
	}
	for i := range latestParts {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseSemverParts(value string) ([3]int, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	base, _, _ := strings.Cut(value, "-")
	segments := strings.Split(base, ".")
	if len(segments) != 3 {
		return [3]int{}, false
	}
	var parts [3]int
	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		parts[i] = n
	}
	return parts, true
}
