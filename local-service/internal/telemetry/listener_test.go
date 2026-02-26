package telemetry

import (
	"context"
	"encoding/binary"
	"math"
	"net"
	"sync"
	"testing"
	"time"
)

func TestListener_ReceiveAndHeartbeat(t *testing.T) {
	// Find two free UDP ports.
	listenConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	listenPort := listenConn.LocalAddr().(*net.UDPAddr).Port
	listenConn.Close()

	heartbeatConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	heartbeatPort := heartbeatConn.LocalAddr().(*net.UDPAddr).Port

	// We'll use heartbeatConn to receive heartbeats and send packets back.
	// Set a read deadline so we don't block forever.
	heartbeatConn.(*net.UDPConn).SetReadDeadline(time.Now().Add(5 * time.Second))

	var mu sync.Mutex
	var receivedPackets []*Packet

	handler := func(pkt *Packet) {
		mu.Lock()
		receivedPackets = append(receivedPackets, pkt)
		mu.Unlock()
	}

	listener := NewListener("127.0.0.1", heartbeatPort, listenPort, handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Run(ctx)
	}()

	// Wait for and verify heartbeat.
	buf := make([]byte, 64)
	n, remoteAddr, err := heartbeatConn.(*net.UDPConn).ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("did not receive heartbeat: %v", err)
	}
	if string(buf[:n]) != "A" {
		t.Errorf("heartbeat = %q, want %q", string(buf[:n]), "A")
	}

	// Now send a valid encrypted telemetry packet back to the listener.
	plain := make([]byte, PacketSize)
	binary.LittleEndian.PutUint32(plain[0:4], magicNumber)
	binary.LittleEndian.PutUint32(plain[ivOffset:ivOffset+4], 0xAABBCCDD)
	binary.LittleEndian.PutUint32(plain[0x70:0x74], 999)                           // PackageID
	binary.LittleEndian.PutUint32(plain[0x124:0x128], 55)                           // CarID
	binary.LittleEndian.PutUint32(plain[0x4C:0x50], math.Float32bits(float32(25.0))) // CarSpeed

	encrypted := encryptPacketForTest(plain)

	// Send from heartbeatConn to listener's address.
	listenerAddr, _ := net.ResolveUDPAddr("udp", remoteAddr.String())
	// Actually we need the listener's address. The listener listens on listenPort.
	listenerUDPAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: listenPort}
	_, err = heartbeatConn.(*net.UDPConn).WriteToUDP(encrypted, listenerUDPAddr)
	if err != nil {
		t.Fatalf("failed to send packet: %v", err)
	}
	_ = listenerAddr // suppress unused

	// Wait a bit for the packet to be processed.
	time.Sleep(200 * time.Millisecond)

	cancel()

	// Check received packets.
	mu.Lock()
	defer mu.Unlock()

	if len(receivedPackets) != 1 {
		t.Fatalf("received %d packets, want 1", len(receivedPackets))
	}

	pkt := receivedPackets[0]
	if pkt.PackageID != 999 {
		t.Errorf("PackageID = %d, want 999", pkt.PackageID)
	}
	if pkt.CarID != 55 {
		t.Errorf("CarID = %d, want 55", pkt.CarID)
	}
	if pkt.CarSpeed != 25.0 {
		t.Errorf("CarSpeed = %f, want 25.0", pkt.CarSpeed)
	}
}
