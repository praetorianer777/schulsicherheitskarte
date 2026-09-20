// Package config reads the region configuration that decides what gets imported.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Region names an area twice: by the prefix of the official municipality key
// (AGS), which is what the accident rows carry, and by a bounding box, which is
// what Overpass understands. The box may be more generous than the prefix — the
// prefix is what actually cuts.
type Region struct {
	Name        string     `yaml:"name"`
	AGSPrefixes []string   `yaml:"ags_prefixes"`
	BBox        [4]float64 `yaml:"bbox"` // minLon, minLat, maxLon, maxLat
}

type Config struct {
	Regions []Region `yaml:"regions"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read region config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse region config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Regions) == 0 {
		return fmt.Errorf("no regions configured")
	}

	var prefixes []string
	for _, r := range c.Regions {
		if r.Name == "" {
			return fmt.Errorf("a region has no name")
		}
		if len(r.AGSPrefixes) == 0 {
			return fmt.Errorf("region %q has no ags_prefixes", r.Name)
		}
		for _, p := range r.AGSPrefixes {
			if p == "" {
				return fmt.Errorf("region %q has an empty ags prefix", r.Name)
			}
			if strings.TrimFunc(p, isDigit) != "" {
				return fmt.Errorf("region %q: ags prefix %q is not a number", r.Name, p)
			}
			prefixes = append(prefixes, p)
		}
		if err := validateBBox(r); err != nil {
			return err
		}
	}

	// Prefixes are matched independently, so a nested pair would import the
	// same accidents once per match. The config comment warns about it; this
	// makes the warning enforceable.
	for i, a := range prefixes {
		for j, b := range prefixes {
			if i != j && strings.HasPrefix(a, b) {
				return fmt.Errorf("ags prefix %q lies inside %q, which would import its accidents twice", a, b)
			}
		}
	}
	return nil
}

func validateBBox(r Region) error {
	minLon, minLat, maxLon, maxLat := r.BBox[0], r.BBox[1], r.BBox[2], r.BBox[3]
	if minLon >= maxLon || minLat >= maxLat {
		return fmt.Errorf("region %q: bbox is empty or inverted; expected minLon, minLat, maxLon, maxLat", r.Name)
	}
	// The data sources are German, so a box outside Germany is a mistake
	// rather than an exotic region — most often latitude and longitude the
	// wrong way round, which passes every generic range check.
	if minLon < germanyMinLon || maxLon > germanyMaxLon || minLat < germanyMinLat || maxLat > germanyMaxLat {
		return fmt.Errorf("region %q: bbox [%g %g %g %g] lies outside Germany; the order is minLon, minLat, maxLon, maxLat",
			r.Name, minLon, minLat, maxLon, maxLat)
	}
	return nil
}

const (
	germanyMinLon = 5.0
	germanyMaxLon = 16.0
	germanyMinLat = 46.0
	germanyMaxLat = 56.0
)

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// Matches reports whether an accident's municipality key belongs to any
// configured region.
func (c *Config) Matches(ags string) bool {
	for _, r := range c.Regions {
		for _, p := range r.AGSPrefixes {
			if strings.HasPrefix(ags, p) {
				return true
			}
		}
	}
	return false
}
