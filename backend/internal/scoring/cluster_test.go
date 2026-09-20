package scoring_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

func at(id int64, x, y float64) scoring.Accident {
	return scoring.Accident{ID: id, X: x, Y: y, Severity: scoring.SeveritySlight, Year: 2025}
}

func TestTwoAccidentsCloseTogetherMakeOneHotspot(t *testing.T) {
	hotspots := scoring.Cluster([]scoring.Accident{at(1, 0, 0), at(2, 30, 0)}, 2025)
	if len(hotspots) != 1 {
		t.Fatalf("produced %d hotspots, want 1", len(hotspots))
	}
	if hotspots[0].Count != 2 {
		t.Errorf("hotspot covers %d accidents, want 2", hotspots[0].Count)
	}
	if hotspots[0].X != 15 || hotspots[0].Y != 0 {
		t.Errorf("centre at %v, %v, want the midpoint 15, 0", hotspots[0].X, hotspots[0].Y)
	}
}

func TestALoneAccidentIsNotAHotspot(t *testing.T) {
	if hotspots := scoring.Cluster([]scoring.Accident{at(1, 0, 0)}, 2025); len(hotspots) != 0 {
		t.Errorf("produced %d hotspots from a single accident", len(hotspots))
	}
}

func TestAccidentsBeyondTheRadiusStayApart(t *testing.T) {
	hotspots := scoring.Cluster([]scoring.Accident{
		at(1, 0, 0), at(2, 10, 0),
		at(3, 500, 0), at(4, 510, 0),
	}, 2025)
	if len(hotspots) != 2 {
		t.Fatalf("produced %d hotspots for two separate places, want 2", len(hotspots))
	}
}

// The reason DBSCAN was rejected. A line of accidents 40 m apart is a street,
// and each of its ends is 400 m from the other; chaining them into one
// "hotspot" is what a road safety inspection cannot act on.
func TestAChainOfAccidentsDoesNotBecomeOneHotspot(t *testing.T) {
	var accidents []scoring.Accident
	for i := 0; i < 11; i++ {
		accidents = append(accidents, at(int64(i+1), float64(i)*40, 0))
	}

	hotspots := scoring.Cluster(accidents, 2025)
	if len(hotspots) < 3 {
		t.Fatalf("a 400 m line of accidents became %d hotspot(s)", len(hotspots))
	}
	for _, h := range hotspots {
		if h.Count > 3 {
			t.Errorf("a hotspot covers %d accidents of a straight line 40 m apart", h.Count)
		}
	}
}

// Whatever the data does, a hotspot has to stay something you can point at.
// This is the property DBSCAN could not give.
func TestNoHotspotIsWiderThanTheRadius(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	var accidents []scoring.Accident
	for i := 0; i < 2000; i++ {
		accidents = append(accidents, at(int64(i+1), random.Float64()*800, random.Float64()*800))
	}

	hotspots := scoring.Cluster(accidents, 2025)
	if len(hotspots) == 0 {
		t.Fatal("no hotspots in a dense field of accidents")
	}
	for _, h := range hotspots {
		// Membership is a circle around the accident the hotspot grew from,
		// while the centre is the mean of its members, so the guaranteed bound
		// is two radii — a hotspot you can still point at on a map.
		if h.Radius > 2*scoring.ClusterRadiusMetres {
			t.Errorf("a hotspot reaches %.1f m from its centre, beyond the guaranteed %.0f m",
				h.Radius, 2*scoring.ClusterRadiusMetres)
		}
		if h.Count < scoring.ClusterMinAccidents {
			t.Errorf("a hotspot holds %d accidents, below the minimum", h.Count)
		}
	}
}

// Two runs over the same data must rank the same, or a fact sheet printed
// twice shows two different top hotspots.
func TestClusteringIsDeterministic(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	var accidents []scoring.Accident
	for i := 0; i < 500; i++ {
		accidents = append(accidents, at(int64(i+1), random.Float64()*400, random.Float64()*400))
	}

	first := scoring.Cluster(accidents, 2025)
	shuffled := make([]scoring.Accident, len(accidents))
	copy(shuffled, accidents)
	random.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	second := scoring.Cluster(shuffled, 2025)

	if len(first) != len(second) {
		t.Fatalf("the same accidents in a different order gave %d and %d hotspots", len(first), len(second))
	}
	for i := range first {
		if first[i].Count != second[i].Count || math.Abs(first[i].Score-second[i].Score) > 1e-9 {
			t.Fatalf("hotspot %d differs between runs: %+v vs %+v", i, first[i], second[i])
		}
	}
}

// Every accident belongs to at most one hotspot, or the same collision would
// be counted twice in two different arguments.
func TestNoAccidentIsCountedTwice(t *testing.T) {
	random := rand.New(rand.NewSource(3))
	var accidents []scoring.Accident
	for i := 0; i < 1000; i++ {
		accidents = append(accidents, at(int64(i+1), random.Float64()*300, random.Float64()*300))
	}

	hotspots := scoring.Cluster(accidents, 2025)
	total := 0
	for _, h := range hotspots {
		total += h.Count
	}
	if total > len(accidents) {
		t.Errorf("hotspots cover %d accidents out of %d", total, len(accidents))
	}
}

// The densest circle wins, so a genuine cluster is not broken up by an
// outlier that happened to be considered first.
func TestTheDensestCircleIsTakenFirst(t *testing.T) {
	// The pair has the lowest ids, so it is considered before the dense group.
	accidents := []scoring.Accident{
		at(1, 0, 0),
		at(2, 45, 0),
		at(10, 500, 0), at(11, 505, 0), at(12, 510, 0), at(13, 495, 0), at(14, 500, 5),
	}

	hotspots := scoring.Cluster(accidents, 2025)
	if len(hotspots) == 0 {
		t.Fatal("no hotspots")
	}
	if hotspots[0].Count != 5 {
		t.Errorf("the first hotspot holds %d accidents; the densest circle holds 5", hotspots[0].Count)
	}
}

func TestHotspotsAreRankedByScore(t *testing.T) {
	accidents := []scoring.Accident{
		{ID: 1, X: 0, Y: 0, Severity: scoring.SeveritySlight, Year: 2025},
		{ID: 2, X: 10, Y: 0, Severity: scoring.SeveritySlight, Year: 2025},
		{ID: 3, X: 500, Y: 0, Severity: scoring.SeverityFatal, Year: 2025, Pedestrian: true},
		{ID: 4, X: 510, Y: 0, Severity: scoring.SeverityFatal, Year: 2025, Pedestrian: true},
	}
	hotspots := scoring.Cluster(accidents, 2025)
	if len(hotspots) != 2 {
		t.Fatalf("produced %d hotspots, want 2", len(hotspots))
	}
	if hotspots[0].Score <= hotspots[1].Score {
		t.Errorf("the two fatal pedestrian accidents did not rank above two slight ones: %v vs %v",
			hotspots[0].Score, hotspots[1].Score)
	}
	if hotspots[0].Fatal != 2 || hotspots[0].Ped != 2 {
		t.Errorf("breakdown of the top hotspot = %d fatal, %d pedestrian", hotspots[0].Fatal, hotspots[0].Ped)
	}
}

func TestYearRangeCoversTheMembers(t *testing.T) {
	hotspots := scoring.Cluster([]scoring.Accident{
		{ID: 1, X: 0, Y: 0, Severity: scoring.SeveritySlight, Year: 2018},
		{ID: 2, X: 10, Y: 0, Severity: scoring.SeveritySlight, Year: 2024},
	}, 2025)
	if len(hotspots) != 1 {
		t.Fatalf("produced %d hotspots", len(hotspots))
	}
	if hotspots[0].FirstYear != 2018 || hotspots[0].LastYear != 2024 {
		t.Errorf("year range = %d..%d, want 2018..2024", hotspots[0].FirstYear, hotspots[0].LastYear)
	}
}

func TestEmptyInputGivesNoHotspots(t *testing.T) {
	if hotspots := scoring.Cluster(nil, 2025); hotspots != nil {
		t.Errorf("produced %d hotspots from no accidents", len(hotspots))
	}
}
