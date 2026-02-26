package trackdetect

import (
	"testing"

	"github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

// makeTrackRef creates a TrackReference with points along a straight line for testing.
func makeTrackRef(name string, startX, startZ, stepX, stepZ float32, numPoints int, elimDist float64) *TrackReference {
	points := make([]ReferencePoint, numPoints)
	for i := 0; i < numPoints; i++ {
		points[i] = ReferencePoint{
			Position: telemetry.Vec3{
				X: startX + float32(i)*stepX,
				Y: 0,
				Z: startZ + float32(i)*stepZ,
			},
			Velocity: telemetry.Vec3{
				X: stepX,
				Y: 0,
				Z: stepZ,
			},
		}
	}
	return &TrackReference{
		Info: TrackInfo{
			Name:             name,
			EliminateDistance: elimDist,
			Slug:             name,
		},
		Points: points,
	}
}

// makePacketAt creates a telemetry Packet at the given position with velocity.
func makePacketAt(x, y, z, vx, vy, vz, speed float32) *telemetry.Packet {
	return &telemetry.Packet{
		Position: telemetry.Vec3{X: x, Y: y, Z: z},
		Velocity: telemetry.Vec3{X: vx, Y: vy, Z: vz},
		CarSpeed: speed,
	}
}

func TestDetector_SingleTrackDetection(t *testing.T) {
	// Create a track along the X axis: (0,0,0) to (99,0,0).
	track := makeTrackRef("test-track", 0, 0, 1.0, 0, 100, 50.0)

	config := DetectorConfig{
		MinPointsBeforeDetection: 5,
		MinHitsForTrack:          3,
		PostDetectionPoints:      2,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}

	detector := NewDetector([]*TrackReference{track}, config)

	// Feed points along the track.
	var result *DetectionResult
	for i := 0; i < 20; i++ {
		pkt := makePacketAt(float32(i), 0, 0, 1.0, 0, 0, 10.0)
		result = detector.AddPoint(pkt)
		if result != nil {
			break
		}
	}

	if result == nil {
		t.Fatal("expected detection result, got nil")
	}
	if result.Track.Info.Name != "test-track" {
		t.Errorf("detected track = %q, want %q", result.Track.Info.Name, "test-track")
	}
	if result.IsReverse {
		t.Error("IsReverse = true, want false")
	}
}

func TestDetector_EliminateFarTrack(t *testing.T) {
	// Track A: along X axis near origin.
	trackA := makeTrackRef("track-a", 0, 0, 1.0, 0, 100, 20.0)
	// Track B: far away at Z=1000.
	trackB := makeTrackRef("track-b", 0, 1000, 1.0, 0, 100, 20.0)

	config := DetectorConfig{
		MinPointsBeforeDetection: 3,
		MinHitsForTrack:          3,
		PostDetectionPoints:      0,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}

	detector := NewDetector([]*TrackReference{trackA, trackB}, config)

	// Feed points near track A -- track B should be eliminated.
	var result *DetectionResult
	for i := 0; i < 30; i++ {
		pkt := makePacketAt(float32(i), 0, 0, 1.0, 0, 0, 10.0)
		result = detector.AddPoint(pkt)
		if result != nil {
			break
		}
	}

	if result == nil {
		t.Fatal("expected detection result, got nil")
	}
	if result.Track.Info.Name != "track-a" {
		t.Errorf("detected track = %q, want %q", result.Track.Info.Name, "track-a")
	}
}

func TestDetector_IgnoreStationaryPoints(t *testing.T) {
	track := makeTrackRef("test-track", 0, 0, 1.0, 0, 100, 50.0)

	config := DetectorConfig{
		MinPointsBeforeDetection: 5,
		MinHitsForTrack:          3,
		PostDetectionPoints:      0,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}

	detector := NewDetector([]*TrackReference{track}, config)

	// Send many stationary points (speed = 0).
	for i := 0; i < 100; i++ {
		pkt := makePacketAt(float32(i), 0, 0, 0, 0, 0, 0)
		result := detector.AddPoint(pkt)
		if result != nil {
			t.Fatal("should not detect with stationary points")
		}
	}

	// Verify pointCount stayed at 0.
	if detector.pointCount != 0 {
		t.Errorf("pointCount = %d, want 0 (stationary points should be ignored)", detector.pointCount)
	}
}

func TestDetector_Reset(t *testing.T) {
	track := makeTrackRef("test-track", 0, 0, 1.0, 0, 100, 50.0)

	config := DetectorConfig{
		MinPointsBeforeDetection: 3,
		MinHitsForTrack:          3,
		PostDetectionPoints:      0,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}

	detector := NewDetector([]*TrackReference{track}, config)

	// Feed some points to advance state.
	for i := 0; i < 10; i++ {
		pkt := makePacketAt(float32(i), 0, 0, 1.0, 0, 0, 10.0)
		detector.AddPoint(pkt)
	}

	// Reset.
	detector.Reset()

	if detector.pointCount != 0 {
		t.Errorf("after Reset, pointCount = %d, want 0", detector.pointCount)
	}

	// Verify candidates are restored.
	activeCount := 0
	for _, c := range detector.candidates {
		if !c.eliminated {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("after Reset, active candidates = %d, want 1", activeCount)
	}

	// Verify detection works again after reset.
	var result *DetectionResult
	for i := 0; i < 20; i++ {
		pkt := makePacketAt(float32(i), 0, 0, 1.0, 0, 0, 10.0)
		result = detector.AddPoint(pkt)
		if result != nil {
			break
		}
	}

	if result == nil {
		t.Fatal("expected detection after reset, got nil")
	}
}

func TestDetector_ReverseDetection(t *testing.T) {
	// Track goes in +X direction.
	track := makeTrackRef("test-track", 0, 0, 1.0, 0, 100, 50.0)

	config := DetectorConfig{
		MinPointsBeforeDetection: 3,
		MinHitsForTrack:          3,
		PostDetectionPoints:      0,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}

	detector := NewDetector([]*TrackReference{track}, config)

	// Feed points going in -X direction (reverse).
	var result *DetectionResult
	for i := 99; i >= 0; i-- {
		pkt := makePacketAt(float32(i), 0, 0, -1.0, 0, 0, 10.0)
		result = detector.AddPoint(pkt)
		if result != nil {
			break
		}
	}

	if result == nil {
		t.Fatal("expected detection result, got nil")
	}
	if !result.IsReverse {
		t.Error("IsReverse = false, want true")
	}
}

func TestAngleBetween(t *testing.T) {
	tests := []struct {
		name string
		a, b telemetry.Vec3
		want float64
		tol  float64
	}{
		{"same direction", telemetry.Vec3{X: 1, Y: 0, Z: 0}, telemetry.Vec3{X: 1, Y: 0, Z: 0}, 0, 0.1},
		{"opposite direction", telemetry.Vec3{X: 1, Y: 0, Z: 0}, telemetry.Vec3{X: -1, Y: 0, Z: 0}, 180, 0.1},
		{"perpendicular", telemetry.Vec3{X: 1, Y: 0, Z: 0}, telemetry.Vec3{X: 0, Y: 1, Z: 0}, 90, 0.1},
		{"zero vector", telemetry.Vec3{X: 0, Y: 0, Z: 0}, telemetry.Vec3{X: 1, Y: 0, Z: 0}, 90, 0.1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := angleBetween(tc.a, tc.b)
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tc.tol {
				t.Errorf("angleBetween(%v, %v) = %f, want %f (tolerance %f)", tc.a, tc.b, got, tc.want, tc.tol)
			}
		})
	}
}
