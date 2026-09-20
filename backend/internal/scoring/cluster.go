package scoring

import (
	"container/heap"
	"math"
	"sort"
)

// Accident is one accident in the metric projection, reduced to what the
// clustering and the score need.
type Accident struct {
	ID         int64
	X, Y       float64 // EPSG:25832, metres
	Severity   int
	Year       int
	Pedestrian bool
	Bike       bool
}

// Hotspot is a circle of ClusterRadiusMetres holding at least
// ClusterMinAccidents accidents.
type Hotspot struct {
	X, Y float64

	// Distance from the centre to the accident furthest from it. Membership is
	// decided by a circle of ClusterRadiusMetres around the accident a hotspot
	// grew from, while the centre reported here is the mean of its members, so
	// this can exceed that radius — never by more than double it, because two
	// members of the same circle are at most two radii apart.
	Radius float64

	Count     int
	Score     float64
	Fatal     int
	Serious   int
	Slight    int
	Ped       int
	Bike      int
	FirstYear int
	LastYear  int
}

// Cluster groups accidents into hotspots.
//
// It repeatedly takes the circle of ClusterRadiusMetres that holds the most
// accidents not yet assigned, and makes that circle a hotspot. Two accidents in
// one hotspot are therefore never more than two radii apart, whatever the data
// looks like.
//
// The obvious alternative, DBSCAN, was tried first and rejected: it joins A to
// B and B to C, so in a town centre it chains along whole streets. On the pilot
// region it produced a "hotspot" 621 m across holding 72 accidents, and those
// blobs ranked at the very top — the one place a fact sheet must not be wrong.
// A road safety inspection is about a junction, not a district.
//
// The result is deterministic: ties are broken by the lowest accident id.
func Cluster(accidents []Accident, referenceYear int) []Hotspot {
	if len(accidents) == 0 {
		return nil
	}

	points := make([]Accident, len(accidents))
	copy(points, accidents)
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })

	grid := newGrid(points, ClusterRadiusMetres)
	assigned := make([]bool, len(points))

	// Seeded with each point's neighbour count. Counts only ever shrink as
	// accidents get assigned, so an entry that is out of date is re-queued with
	// its new count instead of being trusted.
	queue := make(candidates, 0, len(points))
	for i := range points {
		queue = append(queue, candidate{index: i, count: len(grid.within(points, i)), id: points[i].ID})
	}
	heap.Init(&queue)

	var hotspots []Hotspot
	for queue.Len() > 0 {
		best := heap.Pop(&queue).(candidate)
		if assigned[best.index] {
			continue
		}

		members := grid.within(points, best.index)
		available := members[:0]
		for _, m := range members {
			if !assigned[m] {
				available = append(available, m)
			}
		}
		if len(available) < ClusterMinAccidents {
			continue
		}
		// Another circle may have taken some of these accidents since the count
		// was recorded, and a different circle may now be the densest one.
		if len(available) < best.count {
			heap.Push(&queue, candidate{index: best.index, count: len(available), id: best.id})
			continue
		}

		hotspot := Hotspot{FirstYear: math.MaxInt32}
		for _, m := range available {
			assigned[m] = true
			a := points[m]
			hotspot.X += a.X
			hotspot.Y += a.Y
			hotspot.Count++
			hotspot.Score += Score(a.Severity, a.Pedestrian || a.Bike, a.Year, referenceYear)
			switch a.Severity {
			case SeverityFatal:
				hotspot.Fatal++
			case SeveritySerious:
				hotspot.Serious++
			default:
				hotspot.Slight++
			}
			if a.Pedestrian {
				hotspot.Ped++
			}
			if a.Bike {
				hotspot.Bike++
			}
			if a.Year < hotspot.FirstYear {
				hotspot.FirstYear = a.Year
			}
			if a.Year > hotspot.LastYear {
				hotspot.LastYear = a.Year
			}
		}
		hotspot.X /= float64(hotspot.Count)
		hotspot.Y /= float64(hotspot.Count)
		for _, m := range available {
			if d := math.Hypot(points[m].X-hotspot.X, points[m].Y-hotspot.Y); d > hotspot.Radius {
				hotspot.Radius = d
			}
		}
		hotspots = append(hotspots, hotspot)
	}

	sort.Slice(hotspots, func(i, j int) bool { return hotspots[i].Score > hotspots[j].Score })
	return hotspots
}

// A uniform grid with a cell the size of the radius: every neighbour within the
// radius lies in the cell of the point or one of the eight around it.
type grid struct {
	radius  float64
	squared float64
	cells   map[[2]int][]int
}

func newGrid(points []Accident, radius float64) *grid {
	g := &grid{radius: radius, squared: radius * radius, cells: make(map[[2]int][]int, len(points))}
	for i, p := range points {
		key := g.key(p.X, p.Y)
		g.cells[key] = append(g.cells[key], i)
	}
	return g
}

func (g *grid) key(x, y float64) [2]int {
	return [2]int{int(math.Floor(x / g.radius)), int(math.Floor(y / g.radius))}
}

func (g *grid) within(points []Accident, index int) []int {
	centre := points[index]
	key := g.key(centre.X, centre.Y)

	var found []int
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for _, i := range g.cells[[2]int{key[0] + dx, key[1] + dy}] {
				p := points[i]
				if (p.X-centre.X)*(p.X-centre.X)+(p.Y-centre.Y)*(p.Y-centre.Y) <= g.squared {
					found = append(found, i)
				}
			}
		}
	}
	sort.Ints(found)
	return found
}

type candidate struct {
	index int
	count int
	id    int64
}

type candidates []candidate

func (c candidates) Len() int      { return len(c) }
func (c candidates) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c candidates) Less(i, j int) bool {
	if c[i].count != c[j].count {
		return c[i].count > c[j].count
	}
	return c[i].id < c[j].id
}
func (c *candidates) Push(x any) { *c = append(*c, x.(candidate)) }
func (c *candidates) Pop() any {
	old := *c
	last := old[len(old)-1]
	*c = old[:len(old)-1]
	return last
}
