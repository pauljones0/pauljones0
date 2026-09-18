package gocomics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetComicImageURL_OgImage(t *testing.T) {
	expectedURL := "https://featureassets.gocomics.com/assets/test-image-123"
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta property="og:image" content="%s" />
</head>
<body></body>
</html>`, expectedURL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify expected path and headers
		if !strings.Contains(r.URL.Path, "calvinandhobbes") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("User-Agent header was not set")
		}
		if r.Header.Get("Accept") == "" {
			t.Errorf("Accept header was not set")
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	gotURL, err := client.GetComicImageURL("calvinandhobbes", 2026, 9, 17)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotURL != expectedURL {
		t.Errorf("got %q, want %q", gotURL, expectedURL)
	}
}

func TestGetComicImageURL_LDJSON(t *testing.T) {
	expectedURL := "https://featureassets.gocomics.com/assets/ldjson-image-456"
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <script type="application/ld+json">
    {
        "@context": "https://schema.org",
        "@type": "ImageObject",
        "url": "%s",
        "representativeOfPage": true
    }
    </script>
</head>
<body></body>
</html>`, expectedURL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlBody))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	gotURL, err := client.GetComicImageURL("calvinandhobbes", 2026, 9, 17)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotURL != expectedURL {
		t.Errorf("got %q, want %q", gotURL, expectedURL)
	}
}

func TestGetComicImageURL_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("comic not found"))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	_, err := client.GetComicImageURL("calvinandhobbes", 2026, 9, 17)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

func TestGetComicImageURL_NoImageFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head></head><body>No comic here</body></html>"))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	_, err := client.GetComicImageURL("calvinandhobbes", 2026, 9, 17)
	if err == nil {
		t.Fatal("expected error when no image is in HTML, got nil")
	}
}
