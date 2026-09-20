package accidents_test

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/accidents"
)

func readAll(t *testing.T, r io.Reader) []*accidents.Record {
	t.Helper()
	reader, err := accidents.NewReader(r)
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	var records []*accidents.Record
	for {
		rec, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return records
		}
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		records = append(records, rec)
	}
}

func open(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// The 2016 file truncates the road condition column to IstStrasse and drops the
// trailing e of IstSonstige — both artefacts of the shapefile column limit.
func TestReadEarlyHeaderVariant(t *testing.T) {
	records := readAll(t, open(t, "2016.csv"))
	if len(records) != 3 {
		t.Fatalf("read %d records, want 3", len(records))
	}

	first := records[0]
	if first.AGS != "14524280" {
		t.Errorf("AGS = %q, want %q", first.AGS, "14524280")
	}
	if first.Severity != 2 || first.Year != 2016 || first.Month != 5 {
		t.Errorf("unexpected values: severity %d, year %d, month %d", first.Severity, first.Year, first.Month)
	}
	if !first.Pedestrian || first.Bike {
		t.Errorf("involvement flags wrong: pedestrian %v, bike %v", first.Pedestrian, first.Bike)
	}
	if first.Lon != 12.621 || first.Lat != 50.79 {
		t.Errorf("coordinate = %v, %v; the decimal comma was not handled", first.Lon, first.Lat)
	}
	if first.RoadCondition == nil || *first.RoadCondition != 0 {
		t.Errorf("road condition = %v, want 0 from the truncated IstStrasse column", first.RoadCondition)
	}
}

// A Berlin AGS assembled from single-digit parts must keep its leading zeros.
func TestAGSKeepsLeadingZeros(t *testing.T) {
	records := readAll(t, open(t, "2016.csv"))
	if got := records[2].AGS; got != "11000000" {
		t.Errorf("AGS = %q, want %q", got, "11000000")
	}
}

// The published files from 2024 on start with a byte order mark and end with a
// PLST column the schema has no use for.
func TestReadRecentHeaderVariant(t *testing.T) {
	records := readAll(t, open(t, "2025.csv"))
	if len(records) != 2 {
		t.Fatalf("read %d records, want 2", len(records))
	}
	if !records[0].Bike {
		t.Error("IstRad was not read")
	}
	if !records[1].Truck {
		t.Error("IstGkfz was not read")
	}
	if records[1].RoadCondition == nil || *records[1].RoadCondition != 2 {
		t.Errorf("road condition = %v, want 2 from IstStrassenzustand", records[1].RoadCondition)
	}
	if records[0].Lon != 12.623 {
		t.Errorf("longitude = %v; the decimal point was not handled", records[0].Lon)
	}
}

// Both header generations must produce the same digest for the same accident,
// or a re-import after a column rename would duplicate every row.
func TestSourceHashIgnoresColumnOrder(t *testing.T) {
	const a = "ULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n" +
		"14;5;24;280;2024;5;7;3;2;3;4;0;1;1;0;0;12,6210000;50,7900000\n"
	const b = "YGCSWGS84;XGCSWGS84;IstSonstige;IstKrad;IstFuss;IstPKW;IstRad;UTYP1;UART;UKATEGORIE;UWOCHENTAG;USTUNDE;UMONAT;UJAHR;UGEMEINDE;UKREIS;UREGBEZ;ULAND\n" +
		"50.7900000;12.6210000;0;0;1;1;0;4;3;2;3;7;5;2024;280;24;5;14\n"

	first := readAll(t, strings.NewReader(a))
	second := readAll(t, strings.NewReader(b))
	if string(first[0].SourceHash) != string(second[0].SourceHash) {
		t.Error("the same accident produced different digests when the columns were reordered")
	}
}

// The minimal header these fixtures use has no IstGkfz column, which the
// earliest exports also lacked; an absent optional column must read as false
// rather than fail the row.
func TestAbsentOptionalColumnIsNotAnError(t *testing.T) {
	const minimal = "ULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n" +
		"14;5;24;280;2024;5;7;3;2;3;4;0;1;1;0;0;12,6210000;50,7900000\n"
	records := readAll(t, strings.NewReader(minimal))
	if records[0].Truck {
		t.Error("truck is set although the file has no IstGkfz column")
	}
	if records[0].RoadCondition != nil {
		t.Errorf("road condition = %v, want nil when no column carries it", records[0].RoadCondition)
	}
}

func TestUnknownHeaderIsRefused(t *testing.T) {
	_, err := accidents.NewReader(strings.NewReader("foo;bar;baz\n1;2;3\n"))
	if err == nil {
		t.Fatal("an unusable header was accepted")
	}
	// The message has to name what is missing, or the next reporting year costs
	// an afternoon of guessing.
	if !strings.Contains(err.Error(), "ULAND") || !strings.Contains(err.Error(), "found foo") {
		t.Errorf("error does not name the missing and the found columns: %v", err)
	}
}

func TestSwappedCoordinatesAreRefused(t *testing.T) {
	const swapped = "ULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n" +
		"14;5;24;280;2024;5;7;3;2;3;4;0;1;1;0;0;50,7900000;12,6210000\n"
	reader, err := accidents.NewReader(strings.NewReader(swapped))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err == nil {
		t.Fatal("a point outside Germany was accepted")
	}
}

func TestOutOfRangeSeverityIsRefused(t *testing.T) {
	const bad = "ULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n" +
		"14;5;24;280;2024;5;7;3;9;3;4;0;1;1;0;0;12,6210000;50,7900000\n"
	reader, err := accidents.NewReader(strings.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err == nil {
		t.Fatal("severity 9 was accepted although the published range is 1..3")
	}
}

func TestByteOrderMarkIsStripped(t *testing.T) {
	const withBOM = "\xef\xbb\xbfULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n" +
		"14;5;24;280;2024;5;7;3;2;3;4;0;1;1;0;0;12,6210000;50,7900000\n"
	if records := readAll(t, strings.NewReader(withBOM)); len(records) != 1 {
		t.Fatalf("read %d records, want 1", len(records))
	}
}
