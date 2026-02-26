package telemetry

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Vec3 represents a 3D vector (position or velocity).
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

// Packet holds parsed GT7 telemetry data from a decrypted 296-byte packet.
type Packet struct {
	Magic     int32
	Position  Vec3
	Velocity  Vec3
	CarSpeed  float32 // m/s
	PackageID int32
	CurrentLap int16
	TotalLaps  int16
	BestLapTime int32 // milliseconds
	LastLapTime int32 // milliseconds
	CurrentTime int32 // milliseconds
	CarID       int32
	Flags       byte

	// Derived flag fields.
	InRace    bool
	IsPaused  bool
	IsLoading bool
}

// readFloat32 reads a little-endian float32 from data at the given offset.
func readFloat32(data []byte, offset int) float32 {
	bits := binary.LittleEndian.Uint32(data[offset : offset+4])
	return math.Float32frombits(bits)
}

// readInt32 reads a little-endian int32 from data at the given offset.
func readInt32(data []byte, offset int) int32 {
	return int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
}

// readInt16 reads a little-endian int16 from data at the given offset.
func readInt16(data []byte, offset int) int16 {
	return int16(binary.LittleEndian.Uint16(data[offset : offset+2]))
}

// ParsePacket parses a decrypted 296-byte telemetry packet into a Packet struct.
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < PacketSize {
		return nil, fmt.Errorf("data too short: got %d bytes, need %d", len(data), PacketSize)
	}

	flags := data[0x8E]

	p := &Packet{
		Magic:       readInt32(data, 0x00),
		Position:    Vec3{X: readFloat32(data, 0x04), Y: readFloat32(data, 0x08), Z: readFloat32(data, 0x0C)},
		Velocity:    Vec3{X: readFloat32(data, 0x10), Y: readFloat32(data, 0x14), Z: readFloat32(data, 0x18)},
		CarSpeed:    readFloat32(data, 0x4C),
		PackageID:   readInt32(data, 0x70),
		CurrentLap:  readInt16(data, 0x74),
		TotalLaps:   readInt16(data, 0x76),
		BestLapTime: readInt32(data, 0x78),
		LastLapTime: readInt32(data, 0x7C),
		CurrentTime: readInt32(data, 0x80),
		CarID:       readInt32(data, 0x124),
		Flags:       flags,
		InRace:      flags&0x01 != 0,
		IsPaused:    flags&0x02 != 0,
		IsLoading:   flags&0x04 != 0,
	}

	return p, nil
}
