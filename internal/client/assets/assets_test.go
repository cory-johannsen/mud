package assets_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cory-johannsen/mud/internal/client/assets"
)

// githubRelease mirrors the subset of the GitHub Releases API response used by FetchLatestVersion.
type githubRelease struct {
	Assets []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func serveRelease(t *testing.T, release githubRelease) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("failed to encode release: %v", err)
		}
	}))
}

// ---------------------------------------------------------------------------
// ParseVersion
// ---------------------------------------------------------------------------

func TestParseVersion_ValidIntegers(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"  42  ", 42},
		{"42\n", 42},
		{"0", 0},
		{"1000000", 1000000},
	}
	for _, tc := range cases {
		got, err := assets.ParseVersion(tc.input)
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseVersion(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseVersion_InvalidInputs(t *testing.T) {
	cases := []string{"", "  ", "abc", "1.2", "v42", "42abc"}
	for _, tc := range cases {
		_, err := assets.ParseVersion(tc)
		if err == nil {
			t.Errorf("ParseVersion(%q) expected error, got nil", tc)
		}
	}
}

// ---------------------------------------------------------------------------
// FetchLatestVersion — happy path
// ---------------------------------------------------------------------------

func TestFetchLatestVersion_ReturnsCorrectVersion(t *testing.T) {
	release := githubRelease{
		Assets: []githubAsset{
			{
				Name:               "asset-pack-v42.zip",
				BrowserDownloadURL: "https://example.com/asset-pack-v42.zip",
			},
			{
				Name:               "asset-pack-v42.sha256",
				BrowserDownloadURL: "https://example.com/asset-pack-v42.sha256",
			},
		},
	}
	srv := serveRelease(t, release)
	defer srv.Close()

	av, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchLatestVersion unexpected error: %v", err)
	}
	if av.Version != 42 {
		t.Errorf("Version = %d, want 42", av.Version)
	}
	if av.DownloadURL != "https://example.com/asset-pack-v42.zip" {
		t.Errorf("DownloadURL = %q, want zip URL", av.DownloadURL)
	}
	if av.SHA256URL != "https://example.com/asset-pack-v42.sha256" {
		t.Errorf("SHA256URL = %q, want sha256 URL", av.SHA256URL)
	}
}

func TestFetchLatestVersion_ExtraAssetsIgnored(t *testing.T) {
	release := githubRelease{
		Assets: []githubAsset{
			{Name: "README.md", BrowserDownloadURL: "https://example.com/README.md"},
			{Name: "asset-pack-v7.zip", BrowserDownloadURL: "https://example.com/asset-pack-v7.zip"},
			{Name: "asset-pack-v7.sha256", BrowserDownloadURL: "https://example.com/asset-pack-v7.sha256"},
		},
	}
	srv := serveRelease(t, release)
	defer srv.Close()

	av, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchLatestVersion unexpected error: %v", err)
	}
	if av.Version != 7 {
		t.Errorf("Version = %d, want 7", av.Version)
	}
}

// ---------------------------------------------------------------------------
// FetchLatestVersion — ErrNoRelease
// ---------------------------------------------------------------------------

func TestFetchLatestVersion_EmptyAssets_ErrNoRelease(t *testing.T) {
	srv := serveRelease(t, githubRelease{Assets: []githubAsset{}})
	defer srv.Close()

	_, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	if err != assets.ErrNoRelease {
		t.Errorf("expected ErrNoRelease, got %v", err)
	}
}

func TestFetchLatestVersion_NoMatchingAsset_ErrNoRelease(t *testing.T) {
	release := githubRelease{
		Assets: []githubAsset{
			{Name: "something-else.zip", BrowserDownloadURL: "https://example.com/something-else.zip"},
		},
	}
	srv := serveRelease(t, release)
	defer srv.Close()

	_, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	if err != assets.ErrNoRelease {
		t.Errorf("expected ErrNoRelease, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FetchLatestVersion — ErrNetwork
// ---------------------------------------------------------------------------

func TestFetchLatestVersion_Non2xxStatus_ErrNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := assets.FetchLatestVersion(context.Background(), srv.URL)
	if err != assets.ErrNetwork {
		t.Errorf("expected ErrNetwork, got %v", err)
	}
}

func TestFetchLatestVersion_NetworkFailure_ErrNetwork(t *testing.T) {
	// Use a URL that will immediately refuse connections.
	_, err := assets.FetchLatestVersion(context.Background(), "http://127.0.0.1:1")
	if err != assets.ErrNetwork {
		t.Errorf("expected ErrNetwork, got %v", err)
	}
}

func TestFetchLatestVersion_CancelledContext_ErrNetwork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before making the request

	srv := serveRelease(t, githubRelease{
		Assets: []githubAsset{
			{Name: "asset-pack-v1.zip", BrowserDownloadURL: "https://example.com/asset-pack-v1.zip"},
			{Name: "asset-pack-v1.sha256", BrowserDownloadURL: "https://example.com/asset-pack-v1.sha256"},
		},
	})
	defer srv.Close()

	_, err := assets.FetchLatestVersion(ctx, srv.URL)
	if err != assets.ErrNetwork {
		t.Errorf("expected ErrNetwork for cancelled context, got %v", err)
	}
}
