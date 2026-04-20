// Package assets provides asset-version checking for the PixiJS game client.
// It queries a GitHub Releases API endpoint to discover the latest asset pack
// version, download URL, and checksum URL.
package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// ErrNoRelease is returned when the GitHub Releases response contains no asset
// matching the expected asset-pack filename pattern.
var ErrNoRelease = errors.New("assets: no matching release found")

// ErrNetwork is returned for any HTTP or network-level error, including
// non-2xx HTTP status codes and connection failures.
var ErrNetwork = errors.New("assets: network error")

// AssetVersion describes a specific version of the PixiJS asset pack.
type AssetVersion struct {
	// Version is the integer version number parsed from the asset filename.
	Version int
	// DownloadURL is the URL to the zip archive of the asset pack.
	DownloadURL string
	// SHA256URL is the URL to the SHA-256 checksum file for the asset pack.
	SHA256URL string
}

// zipPattern matches filenames of the form "asset-pack-v<N>.zip" and captures
// the numeric version as submatch[1].
var zipPattern = regexp.MustCompile(`^asset-pack-v(\d+)\.zip$`)

// githubRelease is the subset of the GitHub Releases API response that we use.
type githubRelease struct {
	Assets []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// FetchLatestVersion queries the GitHub Releases API at releasesURL and returns
// an AssetVersion describing the latest asset pack.
//
// Preconditions:
//   - ctx must be non-nil.
//   - releasesURL must be a valid HTTP(S) URL pointing to a GitHub Releases API
//     JSON endpoint.
//
// Postconditions:
//   - On success the returned *AssetVersion is non-nil and all fields are
//     populated.
//   - Returns ErrNoRelease if the response contains no asset matching the
//     "asset-pack-v<N>.zip" pattern.
//   - Returns ErrNetwork for any HTTP or network-level failure (including
//     non-2xx HTTP status codes and context cancellation).
func FetchLatestVersion(ctx context.Context, releasesURL string) (*AssetVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, ErrNetwork
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ErrNetwork
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ErrNetwork
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("assets: failed to decode response: %w", err)
	}

	// First pass: locate the zip and parse the version number.
	var version int
	var downloadURL string
	found := false
	for _, asset := range release.Assets {
		m := zipPattern.FindStringSubmatch(asset.Name)
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		version = v
		downloadURL = asset.BrowserDownloadURL
		found = true
		break
	}
	if !found {
		return nil, ErrNoRelease
	}

	// Second pass: locate the corresponding SHA-256 file.
	sha256Name := fmt.Sprintf("asset-pack-v%d.sha256", version)
	var sha256URL string
	for _, asset := range release.Assets {
		if asset.Name == sha256Name {
			sha256URL = asset.BrowserDownloadURL
			break
		}
	}

	return &AssetVersion{
		Version:     version,
		DownloadURL: downloadURL,
		SHA256URL:   sha256URL,
	}, nil
}

// ParseVersion parses a version integer from s after trimming whitespace.
// It returns an error if s (trimmed) is not a valid base-10 integer.
//
// Preconditions:
//   - s may contain leading/trailing whitespace or newlines.
//
// Postconditions:
//   - Returns the parsed integer and nil error on success.
//   - Returns a non-nil error if the trimmed string is empty or non-integer.
func ParseVersion(s string) (int, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("assets: empty version string")
	}
	v, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("assets: invalid version %q: %w", trimmed, err)
	}
	return v, nil
}
