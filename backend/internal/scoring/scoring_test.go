package scoring_test

import (
	"math"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

func TestSeverityOrdersTheOutcomes(t *testing.T) {
	fatal := scoring.Score(scoring.SeverityFatal, false, 2025, 2025)
	serious := scoring.Score(scoring.SeveritySerious, false, 2025, 2025)
	slight := scoring.Score(scoring.SeveritySlight, false, 2025, 2025)

	if !(fatal > serious && serious > slight) {
		t.Fatalf("outcomes are not ordered: fatal %v, serious %v, slight %v", fatal, serious, slight)
	}
	if fatal != 10 || serious != 5 || slight != 1 {
		t.Errorf("weights drifted from the documented 10 / 5 / 1: %v, %v, %v", fatal, serious, slight)
	}
}

// The whole point of the project is the way to school, so an accident that hurt
// somebody on foot or on a bicycle has to weigh more than the same outcome
// between two cars.
func TestVulnerableRoadUsersWeighMore(t *testing.T) {
	withPedestrian := scoring.Score(scoring.SeveritySerious, true, 2025, 2025)
	carsOnly := scoring.Score(scoring.SeveritySerious, false, 2025, 2025)

	if withPedestrian != carsOnly*scoring.WeightVulnerable {
		t.Errorf("pedestrian accident scores %v, car accident %v; the factor is not %v",
			withPedestrian, carsOnly, scoring.WeightVulnerable)
	}
}

func TestRecencyHalvesOverTheHalfLife(t *testing.T) {
	now := scoring.Score(scoring.SeverityFatal, false, 2025, 2025)
	oneHalfLife := scoring.Score(scoring.SeverityFatal, false, 2021, 2025)
	twoHalfLives := scoring.Score(scoring.SeverityFatal, false, 2017, 2025)

	if math.Abs(oneHalfLife-now/2) > 1e-9 {
		t.Errorf("after one half-life the score is %v, want %v", oneHalfLife, now/2)
	}
	if math.Abs(twoHalfLives-now/4) > 1e-9 {
		t.Errorf("after two half-lives the score is %v, want %v", twoHalfLives, now/4)
	}
}

// An old fatal accident must not silently outweigh a recent slight one to an
// absurd degree, and a recent slight one must not outweigh a recent fatal one.
// This pins the shape of the curve, not just its endpoints.
func TestASingleDeathStillOutweighsRecentSlightInjuries(t *testing.T) {
	deathEightYearsAgo := scoring.Score(scoring.SeverityFatal, false, 2017, 2025)
	twoSlightThisYear := 2 * scoring.Score(scoring.SeveritySlight, false, 2025, 2025)

	if deathEightYearsAgo <= twoSlightThisYear {
		t.Errorf("a death eight years ago scores %v, two slight injuries this year %v",
			deathEightYearsAgo, twoSlightThisYear)
	}
}

// A reporting year ahead of the reference would otherwise score above a fresh
// accident, which is meaningless.
func TestAYearAfterTheReferenceIsNotAmplified(t *testing.T) {
	future := scoring.Score(scoring.SeverityFatal, false, 2030, 2025)
	current := scoring.Score(scoring.SeverityFatal, false, 2025, 2025)
	if future != current {
		t.Errorf("a year after the reference scores %v, want the same as the reference year %v", future, current)
	}
}
