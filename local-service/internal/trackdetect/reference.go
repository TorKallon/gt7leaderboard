package trackdetect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

const (
	defaultEliminateDistance = 30.0
)

// TrackInfo holds metadata parsed from a track filename.
type TrackInfo struct {
	Name              string
	Layout            string
	EliminateDistance  float64
	Slug              string
}

// ReferencePoint holds a single reference point from a track recording.
type ReferencePoint struct {
	Position telemetry.Vec3
	Velocity telemetry.Vec3
}

// TrackReference holds a complete track reference including metadata and points.
type TrackReference struct {
	Info   TrackInfo
	Points []ReferencePoint
}

// slugRegex matches characters that should be replaced with dashes in slugs.
var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// generateSlug converts a track name + layout into a URL-friendly slug.
func generateSlug(name, layout string) string {
	s := strings.ToLower(name)
	if layout != "" {
		s += "-" + strings.ToLower(layout)
	}
	s = slugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ParseTrackFilename parses a track filename in the format:
//
//	"Track Name - Layout!PIT-exit-entry-dist!WIDTH-width.gt7track"
//
// The "- Layout", "!PIT-..." and "!WIDTH-..." suffixes are all optional.
func ParseTrackFilename(filename string) TrackInfo {
	// Remove extension.
	base := strings.TrimSuffix(filename, filepath.Ext(filename))

	eliminateDistance := defaultEliminateDistance

	// Extract WIDTH suffix if present.
	if idx := strings.Index(base, "!WIDTH-"); idx >= 0 {
		widthStr := base[idx+7:]
		base = base[:idx]
		if w, err := strconv.ParseFloat(widthStr, 64); err == nil && w > 0 {
			eliminateDistance = w / 2.0
		}
	}

	// Remove PIT suffix if present.
	if idx := strings.Index(base, "!PIT-"); idx >= 0 {
		base = base[:idx]
	}

	// Split name and layout on " - ".
	var name, layout string
	if idx := strings.Index(base, " - "); idx >= 0 {
		name = strings.TrimSpace(base[:idx])
		layout = strings.TrimSpace(base[idx+3:])
	} else {
		name = strings.TrimSpace(base)
	}

	return TrackInfo{
		Name:             name,
		Layout:           layout,
		EliminateDistance: eliminateDistance,
		Slug:             generateSlug(name, layout),
	}
}

// LoadTrackFile loads a .gt7track file containing a sequence of encrypted
// 296-byte telemetry packets. Each packet is decrypted and the XYZ position
// and velocity are extracted into ReferencePoints.
func LoadTrackFile(path string) (*TrackReference, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read track file: %w", err)
	}

	info := ParseTrackFilename(filepath.Base(path))

	numPackets := len(data) / telemetry.PacketSize
	if numPackets == 0 {
		return nil, fmt.Errorf("track file too small: %d bytes", len(data))
	}

	points := make([]ReferencePoint, 0, numPackets)
	for i := 0; i < numPackets; i++ {
		offset := i * telemetry.PacketSize
		chunk := data[offset : offset+telemetry.PacketSize]

		decrypted, err := telemetry.DecryptPacket(chunk)
		if err != nil {
			// Skip packets that fail to decrypt.
			continue
		}

		pkt, err := telemetry.ParsePacket(decrypted)
		if err != nil {
			continue
		}

		points = append(points, ReferencePoint{
			Position: pkt.Position,
			Velocity: pkt.Velocity,
		})
	}

	return &TrackReference{
		Info:   info,
		Points: points,
	}, nil
}

// LoadAllTracks loads all .gt7track files from a directory.
func LoadAllTracks(dir string) ([]*TrackReference, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read track directory: %w", err)
	}

	var tracks []*TrackReference
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".gt7track") {
			continue
		}

		track, err := LoadTrackFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			// Skip files that fail to load.
			continue
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}
