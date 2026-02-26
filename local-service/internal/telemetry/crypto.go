package telemetry

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/salsa20"
)

const (
	// PacketSize is the size of a GT7 telemetry packet in bytes.
	PacketSize = 296

	// magicNumber is the expected magic value after decryption (ASCII "G7S0").
	magicNumber uint32 = 0x47375330

	// ivOffset is the byte offset where the 4-byte IV seed is located in the packet.
	ivOffset = 0x40

	// ivXORMask is XOR'd with the IV seed bytes.
	ivXORMask uint32 = 0xDEADBEAF
)

// salsaKey is the first 32 bytes of the GT7 Simulator Interface key string.
var salsaKey [32]byte

func init() {
	keyStr := "Simulator Interface Packet GT7 ver 0.0"
	copy(salsaKey[:], keyStr[:32])
}

// DecryptPacket decrypts a GT7 telemetry packet using Salsa20.
// The packet must be exactly PacketSize (296) bytes.
// Returns the decrypted packet or an error if the packet is invalid.
func DecryptPacket(data []byte) ([]byte, error) {
	if len(data) < PacketSize {
		return nil, fmt.Errorf("packet too short: got %d bytes, need %d", len(data), PacketSize)
	}

	// Extract IV seed from offset 0x40 (4 bytes, little-endian).
	ivSeed := binary.LittleEndian.Uint32(data[ivOffset : ivOffset+4])

	// XOR with magic mask.
	ivSeed ^= ivXORMask

	// Build 8-byte nonce: ivSeed as little-endian uint32 in first 4 bytes, rest zero.
	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[:4], ivSeed)

	// Decrypt using Salsa20.
	out := make([]byte, PacketSize)
	copy(out, data[:PacketSize])
	salsa20.XORKeyStream(out, out, nonce[:], &salsaKey)

	// Validate magic number at offset 0x00.
	magic := binary.LittleEndian.Uint32(out[0:4])
	if magic != magicNumber {
		return nil, errors.New("invalid magic number after decryption")
	}

	return out, nil
}
