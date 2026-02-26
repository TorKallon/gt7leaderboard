package trackdetect

import (
	"testing"
)

func TestParseTrackFilename_Full(t *testing.T) {
	info := ParseTrackFilename("Tsukuba Circuit - Full Course!PIT-100-200-50!WIDTH-60.gt7track")

	if info.Name != "Tsukuba Circuit" {
		t.Errorf("Name = %q, want %q", info.Name, "Tsukuba Circuit")
	}
	if info.Layout != "Full Course" {
		t.Errorf("Layout = %q, want %q", info.Layout, "Full Course")
	}
	if info.EliminateDistance != 30.0 {
		t.Errorf("EliminateDistance = %f, want 30.0", info.EliminateDistance)
	}
	if info.Slug != "tsukuba-circuit-full-course" {
		t.Errorf("Slug = %q, want %q", info.Slug, "tsukuba-circuit-full-course")
	}
}

func TestParseTrackFilename_NoLayout(t *testing.T) {
	info := ParseTrackFilename("Nurburgring Nordschleife!PIT-1-2-3!WIDTH-80.gt7track")

	if info.Name != "Nurburgring Nordschleife" {
		t.Errorf("Name = %q, want %q", info.Name, "Nurburgring Nordschleife")
	}
	if info.Layout != "" {
		t.Errorf("Layout = %q, want empty string", info.Layout)
	}
	if info.EliminateDistance != 40.0 {
		t.Errorf("EliminateDistance = %f, want 40.0 (80/2)", info.EliminateDistance)
	}
	if info.Slug != "nurburgring-nordschleife" {
		t.Errorf("Slug = %q, want %q", info.Slug, "nurburgring-nordschleife")
	}
}

func TestParseTrackFilename_NoSuffixes(t *testing.T) {
	info := ParseTrackFilename("Deep Forest Raceway.gt7track")

	if info.Name != "Deep Forest Raceway" {
		t.Errorf("Name = %q, want %q", info.Name, "Deep Forest Raceway")
	}
	if info.Layout != "" {
		t.Errorf("Layout = %q, want empty", info.Layout)
	}
	if info.EliminateDistance != defaultEliminateDistance {
		t.Errorf("EliminateDistance = %f, want %f", info.EliminateDistance, defaultEliminateDistance)
	}
	if info.Slug != "deep-forest-raceway" {
		t.Errorf("Slug = %q, want %q", info.Slug, "deep-forest-raceway")
	}
}

func TestParseTrackFilename_WithLayout_NoSuffixes(t *testing.T) {
	info := ParseTrackFilename("Suzuka Circuit - East Course.gt7track")

	if info.Name != "Suzuka Circuit" {
		t.Errorf("Name = %q, want %q", info.Name, "Suzuka Circuit")
	}
	if info.Layout != "East Course" {
		t.Errorf("Layout = %q, want %q", info.Layout, "East Course")
	}
	if info.Slug != "suzuka-circuit-east-course" {
		t.Errorf("Slug = %q, want %q", info.Slug, "suzuka-circuit-east-course")
	}
}

func TestParseTrackFilename_WidthOnly(t *testing.T) {
	info := ParseTrackFilename("Test Track!WIDTH-100.gt7track")

	if info.Name != "Test Track" {
		t.Errorf("Name = %q, want %q", info.Name, "Test Track")
	}
	if info.EliminateDistance != 50.0 {
		t.Errorf("EliminateDistance = %f, want 50.0 (100/2)", info.EliminateDistance)
	}
}

func TestParseTrackFilename_PitOnly(t *testing.T) {
	info := ParseTrackFilename("Le Mans - Circuit de la Sarthe!PIT-10-20-5.gt7track")

	if info.Name != "Le Mans" {
		t.Errorf("Name = %q, want %q", info.Name, "Le Mans")
	}
	if info.Layout != "Circuit de la Sarthe" {
		t.Errorf("Layout = %q, want %q", info.Layout, "Circuit de la Sarthe")
	}
	if info.EliminateDistance != defaultEliminateDistance {
		t.Errorf("EliminateDistance = %f, want %f (default)", info.EliminateDistance, defaultEliminateDistance)
	}
}

func TestSlugGeneration(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		want   string
	}{
		{"Tokyo Expressway", "South Inner Loop", "tokyo-expressway-south-inner-loop"},
		{"Spa-Francorchamps", "", "spa-francorchamps"},
		{"Circuit de la Sarthe", "", "circuit-de-la-sarthe"},
		{"Mount Panorama", "Motor Racing Circuit", "mount-panorama-motor-racing-circuit"},
	}

	for _, tc := range tests {
		got := generateSlug(tc.name, tc.layout)
		if got != tc.want {
			t.Errorf("generateSlug(%q, %q) = %q, want %q", tc.name, tc.layout, got, tc.want)
		}
	}
}
