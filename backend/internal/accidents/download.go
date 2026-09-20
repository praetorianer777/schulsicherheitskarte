package accidents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The statistical offices publish the nationwide files through OpenGeodata.NRW.
// Licence: dl-de/by-2-0.
const DownloadURLTemplate = "https://www.opengeodata.nrw.de/produkte/transport_verkehr/unfallatlas/Unfallorte%d_EPSG25832_CSV.zip"

// A named agent is what the publisher asks for and what lets them see who is
// causing load.
const userAgent = "schulsicherheitskarte/0.1 (+https://github.com/praetorianer777/schulsicherheitskarte)"

type Download struct {
	Path     string
	Checksum string
	Cached   bool // the server confirmed the cached copy is still current
}

// Fetch downloads the archive for one reporting year into cacheDir. A cached
// copy is revalidated with its ETag rather than downloaded again — the archives
// are tens of megabytes and change once a year.
func Fetch(ctx context.Context, client *http.Client, year int, cacheDir string) (*Download, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	archive := filepath.Join(cacheDir, fmt.Sprintf("Unfallorte%d_EPSG25832_CSV.zip", year))
	etagFile := archive + ".etag"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(DownloadURLTemplate, year), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	cachedETag, _ := os.ReadFile(etagFile)
	if _, err := os.Stat(archive); err == nil && len(cachedETag) > 0 {
		req.Header.Set("If-None-Match", strings.TrimSpace(string(cachedETag)))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %d: %w", year, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		sum, err := checksumFile(archive)
		if err != nil {
			return nil, err
		}
		return &Download{Path: archive, Checksum: sum, Cached: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %d: server answered %s", year, resp.Status)
	}

	// Written beside the target and renamed, so an interrupted download cannot
	// leave a truncated archive that looks complete on the next run.
	tmp, err := os.CreateTemp(cacheDir, "download-*.part")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("download %d: %w", year, err)
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), archive); err != nil {
		return nil, err
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		_ = os.WriteFile(etagFile, []byte(etag), 0o644)
	}
	return &Download{Path: archive, Checksum: hex.EncodeToString(hash.Sum(nil))}, nil
}

func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DefaultClient is deliberately patient: the archives are large and the
// publisher is a public service, not a CDN.
func DefaultClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute}
}
