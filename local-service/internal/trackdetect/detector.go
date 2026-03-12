package trackdetect

import (
	"math"

	"github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
)

// DetectorConfig holds tunable parameters for the track detection algorithm.
type DetectorConfig struct {
	MinPointsBeforeDetection int     // minimum telemetry points before detection starts
	MinHitsForTrack          int     // minimum reference point hits to consider a candidate valid
	PostDetectionPoints      int     // additional points to receive after narrowing to 1 candidate
	MaxGapLength             int     // max consecutive missed reference points before elimination
	ValidAngleDeg            float64 // max forward angle in degrees (reverse is 180 - this)
}

// DefaultConfig returns the default detector configuration.
func DefaultConfig() DetectorConfig {
	return DetectorConfig{
		MinPointsBeforeDetection: 300,
		MinHitsForTrack:          5,
		PostDetectionPoints:      60,
		MaxGapLength:             10,
		ValidAngleDeg:            15.0,
	}
}

// DetectionResult holds the outcome of a successful track detection.
type DetectionResult struct {
	Track     *TrackReference
	IsReverse bool
}

// candidateState tracks the state of a single candidate track during detection.
type candidateState struct {
	track         *TrackReference
	lastHitIdx    int  // index of last hit reference point (-1 = none)
	hitCount      int  // number of reference point hits
	forwardCount  int  // velocity dot products suggesting forward direction
	reverseCount  int  // velocity dot products suggesting reverse direction
	eliminated    bool
}

// Detector implements elimination-based nearest-neighbor track detection.
type Detector struct {
	config     DetectorConfig
	candidates []*candidateState
	pointCount int

	// Set when exactly one candidate remains and has enough hits.
	pendingResult    *DetectionResult
	postDetectCount  int
}

// NewDetector creates a new Detector with the given track references and configuration.
func NewDetector(tracks []*TrackReference, config DetectorConfig) *Detector {
	candidates := make([]*candidateState, len(tracks))
	for i, tr := range tracks {
		candidates[i] = &candidateState{
			track:      tr,
			lastHitIdx: -1,
		}
	}
	return &Detector{
		config:     config,
		candidates: candidates,
	}
}

// ReloadTracks replaces the track references and resets all detection state.
func (d *Detector) ReloadTracks(tracks []*TrackReference) {
	candidates := make([]*candidateState, len(tracks))
	for i, tr := range tracks {
		candidates[i] = &candidateState{
			track:      tr,
			lastHitIdx: -1,
		}
	}
	d.candidates = candidates
	d.pointCount = 0
	d.pendingResult = nil
	d.postDetectCount = 0
}

// Reset clears all detection state so the detector can be reused.
func (d *Detector) Reset() {
	for _, c := range d.candidates {
		c.lastHitIdx = -1
		c.hitCount = 0
		c.forwardCount = 0
		c.reverseCount = 0
		c.eliminated = false
	}
	d.pointCount = 0
	d.pendingResult = nil
	d.postDetectCount = 0
}

// AddPoint processes a telemetry packet and returns a DetectionResult if the
// track has been identified, or nil if detection is still in progress.
func (d *Detector) AddPoint(pkt *telemetry.Packet) *DetectionResult {
	// Ignore packets that aren't from active racing — loading screens and
	// menus can report non-zero speed with invalid position data, which
	// would incorrectly eliminate track candidates.
	if pkt.CarSpeed <= 0 || !pkt.InRace || pkt.IsLoading {
		return nil
	}

	d.pointCount++

	// If we already have a pending result, count post-detection points.
	if d.pendingResult != nil {
		d.postDetectCount++
		if d.postDetectCount >= d.config.PostDetectionPoints {
			return d.pendingResult
		}
		return nil
	}

	// Don't start elimination until we have enough points.
	if d.pointCount < d.config.MinPointsBeforeDetection {
		return nil
	}

	pos := pkt.Position
	vel := pkt.Velocity

	// Process each non-eliminated candidate.
	for _, c := range d.candidates {
		if c.eliminated {
			continue
		}

		// Find closest reference point (3D Euclidean distance).
		if len(c.track.Points) == 0 {
			c.eliminated = true
			continue
		}
		closestIdx, closestDist := findClosestPoint(c.track.Points, pos)

		// Eliminate if too far.
		elimDist := c.track.Info.EliminateDistance
		if closestDist > elimDist {
			c.eliminated = true
			continue
		}

		// Check gap: if we had a previous hit, check consecutive missed reference points.
		if c.lastHitIdx >= 0 {
			gap := absInt(closestIdx - c.lastHitIdx)
			// Wrap around for circular tracks.
			wrapGap := len(c.track.Points) - gap
			if wrapGap < gap {
				gap = wrapGap
			}
			if gap > d.config.MaxGapLength {
				c.eliminated = true
				continue
			}
		}

		c.lastHitIdx = closestIdx
		c.hitCount++

		// Track direction via velocity dot product.
		refVel := c.track.Points[closestIdx].Velocity
		angle := angleBetween(vel, refVel)
		if angle < d.config.ValidAngleDeg {
			c.forwardCount++
		} else if angle > (180.0 - d.config.ValidAngleDeg) {
			c.reverseCount++
		}
	}

	// Count remaining candidates.
	var remaining []*candidateState
	for _, c := range d.candidates {
		if !c.eliminated {
			remaining = append(remaining, c)
		}
	}

	// Check if we have exactly one candidate with enough hits.
	if len(remaining) == 1 && remaining[0].hitCount >= d.config.MinHitsForTrack {
		c := remaining[0]
		d.pendingResult = &DetectionResult{
			Track:     c.track,
			IsReverse: c.reverseCount > c.forwardCount,
		}
		d.postDetectCount = 0
		// If PostDetectionPoints is 0, return immediately.
		if d.config.PostDetectionPoints <= 0 {
			return d.pendingResult
		}
	}

	return nil
}

// findClosestPoint finds the closest reference point to the given position.
// Returns the index and the Euclidean distance.
func findClosestPoint(points []ReferencePoint, pos telemetry.Vec3) (int, float64) {
	bestIdx := 0
	bestDist := math.MaxFloat64

	for i, rp := range points {
		dx := float64(pos.X - rp.Position.X)
		dy := float64(pos.Y - rp.Position.Y)
		dz := float64(pos.Z - rp.Position.Z)
		dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	return bestIdx, bestDist
}

// angleBetween computes the angle in degrees between two Vec3 vectors.
func angleBetween(a, b telemetry.Vec3) float64 {
	dot := float64(a.X*b.X + a.Y*b.Y + a.Z*b.Z)
	magA := math.Sqrt(float64(a.X*a.X + a.Y*a.Y + a.Z*a.Z))
	magB := math.Sqrt(float64(b.X*b.X + b.Y*b.Y + b.Z*b.Z))

	if magA == 0 || magB == 0 {
		return 90.0 // undefined, return neutral
	}

	cosTheta := dot / (magA * magB)
	// Clamp to [-1, 1] to avoid NaN from floating point errors.
	if cosTheta > 1 {
		cosTheta = 1
	}
	if cosTheta < -1 {
		cosTheta = -1
	}

	return math.Acos(cosTheta) * 180.0 / math.Pi
}

// absInt returns the absolute value of an integer.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
