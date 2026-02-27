package telemetry

import (
	"encoding/binary"
	"testing"

	"golang.org/x/crypto/salsa20"
)

// encryptPacketForTest takes a plaintext 296-byte buffer (with valid magic already set)
// and encrypts it using the same Salsa20 scheme GT7 uses. The IV seed at offset 0x40
// must already be set in the plaintext; this function reads it, XORs with the mask,
// builds the nonce, and encrypts in-place, then restores the IV seed bytes in the
// ciphertext (since those bytes are read before decryption from the encrypted packet).
func encryptPacketForTest(plain []byte) []byte {
	if len(plain) < PacketSize {
		panic("plaintext too short for encryption")
	}

	// Read IV seed that we placed in the plaintext at 0x40.
	iv1 := binary.LittleEndian.Uint32(plain[ivOffset : ivOffset+4])

	// Build nonce the same way DecryptPacket does.
	iv2 := iv1 ^ ivXORMask
	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[:4], iv2)
	binary.LittleEndian.PutUint32(nonce[4:], iv1)

	// Encrypt (Salsa20 XOR is symmetric).
	out := make([]byte, PacketSize)
	copy(out, plain[:PacketSize])
	salsa20.XORKeyStream(out, out, nonce[:], &salsaKey)

	// Restore IV seed in the ciphertext so DecryptPacket can read it.
	binary.LittleEndian.PutUint32(out[ivOffset:ivOffset+4], iv1)

	return out
}

func TestDecryptPacket_Valid(t *testing.T) {
	plain := make([]byte, PacketSize)
	// Set magic number.
	binary.LittleEndian.PutUint32(plain[0:4], magicNumber)
	// Set an IV seed at offset 0x40.
	binary.LittleEndian.PutUint32(plain[ivOffset:ivOffset+4], 0x12345678)
	// Set a known value so we can verify round-trip.
	binary.LittleEndian.PutUint32(plain[0x70:0x74], 42) // PackageID

	encrypted := encryptPacketForTest(plain)
	decrypted, err := DecryptPacket(encrypted)
	if err != nil {
		t.Fatalf("DecryptPacket failed: %v", err)
	}

	// Verify magic.
	magic := binary.LittleEndian.Uint32(decrypted[0:4])
	if magic != magicNumber {
		t.Errorf("magic = 0x%08X, want 0x%08X", magic, magicNumber)
	}

	// Verify PackageID survived round-trip.
	pkgID := binary.LittleEndian.Uint32(decrypted[0x70:0x74])
	if pkgID != 42 {
		t.Errorf("PackageID = %d, want 42", pkgID)
	}
}

func TestDecryptPacket_TooShort(t *testing.T) {
	data := make([]byte, 100)
	_, err := DecryptPacket(data)
	if err == nil {
		t.Fatal("expected error for short packet, got nil")
	}
}

func TestDecryptPacket_BadMagic(t *testing.T) {
	plain := make([]byte, PacketSize)
	// Set wrong magic number.
	binary.LittleEndian.PutUint32(plain[0:4], 0xDEADDEAD)
	binary.LittleEndian.PutUint32(plain[ivOffset:ivOffset+4], 0x12345678)

	encrypted := encryptPacketForTest(plain)
	_, err := DecryptPacket(encrypted)
	if err == nil {
		t.Fatal("expected error for bad magic, got nil")
	}
}
