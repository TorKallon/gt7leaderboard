package telemetry

import (
	"encoding/binary"
	"math"
	"testing"
)

// makeTestPacketBytes creates a valid 296-byte decrypted packet buffer with
// the magic number set. The provided function can customize any bytes.
func makeTestPacketBytes(fn func(data []byte)) []byte {
	data := make([]byte, PacketSize)
	binary.LittleEndian.PutUint32(data[0:4], magicNumber)
	if fn != nil {
		fn(data)
	}
	return data
}

func putFloat32(data []byte, offset int, v float32) {
	binary.LittleEndian.PutUint32(data[offset:offset+4], math.Float32bits(v))
}

func TestParsePacket_AllFields(t *testing.T) {
	data := makeTestPacketBytes(func(d []byte) {
		putFloat32(d, 0x04, 1.0)   // PositionX
		putFloat32(d, 0x08, 2.0)   // PositionY
		putFloat32(d, 0x0C, 3.0)   // PositionZ
		putFloat32(d, 0x10, 4.0)   // VelocityX
		putFloat32(d, 0x14, 5.0)   // VelocityY
		putFloat32(d, 0x18, 6.0)   // VelocityZ
		putFloat32(d, 0x4C, 33.5)  // CarSpeed
		binary.LittleEndian.PutUint32(d[0x70:0x74], 100) // PackageID
		binary.LittleEndian.PutUint16(d[0x74:0x76], 3)   // CurrentLap
		binary.LittleEndian.PutUint16(d[0x76:0x78], 5)   // TotalLaps
		binary.LittleEndian.PutUint32(d[0x78:0x7C], 90000)  // BestLapTime
		binary.LittleEndian.PutUint32(d[0x7C:0x80], 95000)  // LastLapTime
		binary.LittleEndian.PutUint32(d[0x80:0x84], 120000) // CurrentTime
		binary.LittleEndian.PutUint32(d[0x124:0x128], 42)   // CarID
	})

	pkt, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket failed: %v", err)
	}

	if pkt.Magic != int32(magicNumber) {
		t.Errorf("Magic = 0x%08X, want 0x%08X", uint32(pkt.Magic), magicNumber)
	}
	if pkt.Position.X != 1.0 || pkt.Position.Y != 2.0 || pkt.Position.Z != 3.0 {
		t.Errorf("Position = %v, want {1 2 3}", pkt.Position)
	}
	if pkt.Velocity.X != 4.0 || pkt.Velocity.Y != 5.0 || pkt.Velocity.Z != 6.0 {
		t.Errorf("Velocity = %v, want {4 5 6}", pkt.Velocity)
	}
	if pkt.CarSpeed != 33.5 {
		t.Errorf("CarSpeed = %f, want 33.5", pkt.CarSpeed)
	}
	if pkt.PackageID != 100 {
		t.Errorf("PackageID = %d, want 100", pkt.PackageID)
	}
	if pkt.CurrentLap != 3 {
		t.Errorf("CurrentLap = %d, want 3", pkt.CurrentLap)
	}
	if pkt.TotalLaps != 5 {
		t.Errorf("TotalLaps = %d, want 5", pkt.TotalLaps)
	}
	if pkt.BestLapTime != 90000 {
		t.Errorf("BestLapTime = %d, want 90000", pkt.BestLapTime)
	}
	if pkt.LastLapTime != 95000 {
		t.Errorf("LastLapTime = %d, want 95000", pkt.LastLapTime)
	}
	if pkt.CurrentTime != 120000 {
		t.Errorf("CurrentTime = %d, want 120000", pkt.CurrentTime)
	}
	if pkt.CarID != 42 {
		t.Errorf("CarID = %d, want 42", pkt.CarID)
	}
}

func TestParsePacket_Flags(t *testing.T) {
	tests := []struct {
		name      string
		flags     byte
		inRace    bool
		isPaused  bool
		isLoading bool
	}{
		{"no flags", 0x00, false, false, false},
		{"InRace only", 0x01, true, false, false},
		{"IsPaused only", 0x02, false, true, false},
		{"IsLoading only", 0x04, false, false, true},
		{"all flags", 0x07, true, true, true},
		{"InRace+IsLoading", 0x05, true, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := makeTestPacketBytes(func(d []byte) {
				d[0x8E] = tc.flags
			})
			pkt, err := ParsePacket(data)
			if err != nil {
				t.Fatalf("ParsePacket failed: %v", err)
			}
			if pkt.InRace != tc.inRace {
				t.Errorf("InRace = %v, want %v", pkt.InRace, tc.inRace)
			}
			if pkt.IsPaused != tc.isPaused {
				t.Errorf("IsPaused = %v, want %v", pkt.IsPaused, tc.isPaused)
			}
			if pkt.IsLoading != tc.isLoading {
				t.Errorf("IsLoading = %v, want %v", pkt.IsLoading, tc.isLoading)
			}
		})
	}
}

func TestParsePacket_TooShort(t *testing.T) {
	data := make([]byte, 100)
	_, err := ParsePacket(data)
	if err == nil {
		t.Fatal("expected error for short data, got nil")
	}
}
