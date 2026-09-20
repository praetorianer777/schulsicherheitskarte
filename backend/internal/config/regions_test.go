package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/config"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "regions.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadsTheShippedConfiguration(t *testing.T) {
	cfg, err := config.Load("../../../regions.yaml")
	if err != nil {
		t.Fatalf("the configuration shipped with the repository does not load: %v", err)
	}
	if !cfg.Matches("14524280") {
		t.Error("St. Egidien does not match the configured pilot region")
	}
	if cfg.Matches("11000000") {
		t.Error("Berlin matches although the pilot region is Landkreis Zwickau")
	}
}

// Nested prefixes are matched independently, so the inner one's accidents would
// be counted twice. The comment in regions.yaml warns about it; this is what
// makes the warning hold.
func TestNestedPrefixesAreRefused(t *testing.T) {
	path := write(t, `
regions:
  - name: Landkreis Zwickau
    ags_prefixes: ["14524"]
    bbox: [12.2, 50.5, 12.8, 50.9]
  - name: St. Egidien
    ags_prefixes: ["14524280"]
    bbox: [12.5, 50.7, 12.7, 50.8]
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("a nested prefix pair was accepted")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("error does not explain the consequence: %v", err)
	}
}

func TestSwappedBoundingBoxIsRefused(t *testing.T) {
	path := write(t, `
regions:
  - name: Swapped
    ags_prefixes: ["14524"]
    bbox: [50.5, 12.2, 50.9, 12.8]
`)
	if _, err := config.Load(path); err == nil {
		t.Fatal("a bounding box with latitude and longitude swapped was accepted")
	}
}

func TestInvertedBoundingBoxIsRefused(t *testing.T) {
	path := write(t, `
regions:
  - name: Inverted
    ags_prefixes: ["14524"]
    bbox: [12.8, 50.9, 12.2, 50.5]
`)
	if _, err := config.Load(path); err == nil {
		t.Fatal("an inverted bounding box was accepted")
	}
}

func TestEmptyConfigurationIsRefused(t *testing.T) {
	if _, err := config.Load(write(t, "regions: []\n")); err == nil {
		t.Fatal("a configuration without regions was accepted")
	}
}

func TestNonNumericPrefixIsRefused(t *testing.T) {
	path := write(t, `
regions:
  - name: Bad
    ags_prefixes: ["Zwickau"]
    bbox: [12.2, 50.5, 12.8, 50.9]
`)
	if _, err := config.Load(path); err == nil {
		t.Fatal("a non-numeric ags prefix was accepted")
	}
}

func TestMatchesOnlyOnPrefix(t *testing.T) {
	cfg := &config.Config{Regions: []config.Region{{
		Name: "Landkreis Zwickau", AGSPrefixes: []string{"14524"},
		BBox: [4]float64{12.2, 50.5, 12.8, 50.9},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for ags, want := range map[string]bool{
		"14524280": true,
		"14524000": true,
		"14523000": false,
		"1452":     false,
	} {
		if got := cfg.Matches(ags); got != want {
			t.Errorf("Matches(%q) = %v, want %v", ags, got, want)
		}
	}
}
