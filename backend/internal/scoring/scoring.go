// Package scoring turns accidents into hotspots and gives each one a number a
// parent representative has to be able to explain in a meeting.
package scoring

import "math"

// The weights are the whole argument of this project, so they live here, in one
// place, and are documented in the README with their reasoning. Changing them
// changes every number on every fact sheet.
const (
	// A death is not five slight injuries. The ratio is a judgement, and it is
	// stated rather than hidden in a query.
	WeightFatal   = 10.0
	WeightSerious = 5.0
	WeightSlight  = 1.0

	// This project is about the way to school. An accident that hurt somebody
	// on foot or on a bicycle says more about that than a rear-end collision
	// between two cars at the same spot.
	WeightVulnerable = 3.0

	// Infrastructure changes, traffic changes, and a crossing built in 2018 is
	// not answered by what happened in 2016. Four years is short enough to
	// follow such a change and long enough that a single quiet year does not
	// erase a known danger.
	HalfLifeYears = 4.0

	// Two accidents within 50 m are at the same place as far as a road safety
	// inspection is concerned. A single accident is not a hotspot — it is an
	// accident, and calling it a pattern is what gets a fact sheet dismissed.
	ClusterRadiusMetres = 50.0
	ClusterMinAccidents = 2

	// The radius of the one figure the overview shows per institution. It is
	// the institution page's default radius, so a click on a point opens a page
	// that starts with the number the point stood for.
	NearbyRadiusMetres = 500
)

// Severity codes as published in the Unfallatlas (UKATEGORIE).
const (
	SeverityFatal   = 1
	SeveritySerious = 2
	SeveritySlight  = 3
)

// SeverityWeight is the weight of one accident's outcome.
func SeverityWeight(severity int) float64 {
	switch severity {
	case SeverityFatal:
		return WeightFatal
	case SeveritySerious:
		return WeightSerious
	default:
		return WeightSlight
	}
}

// Score weights one accident by outcome, by who was hurt, and by how long ago
// it happened.
//
// referenceYear is the most recent reporting year in the data, not the current
// year: the score of a hotspot then depends on the data, not on the day
// somebody opens the page, and two people reading the same fact sheet a month
// apart see the same number.
func Score(severity int, vulnerable bool, year, referenceYear int) float64 {
	weight := SeverityWeight(severity)
	if vulnerable {
		weight *= WeightVulnerable
	}
	age := float64(referenceYear - year)
	if age < 0 {
		age = 0
	}
	return weight * math.Pow(0.5, age/HalfLifeYears)
}
