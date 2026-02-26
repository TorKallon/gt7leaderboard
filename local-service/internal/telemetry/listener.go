package telemetry

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	heartbeatInterval = 10 * time.Second
	readBufferSize    = 2048
)

// PacketHandler is a callback invoked for each successfully decrypted and parsed packet.
type PacketHandler func(*Packet)

// Listener receives GT7 telemetry data via UDP.
// It sends periodic heartbeat messages to the PS5 and listens for encrypted
// telemetry packets, decrypting and parsing them before passing to the handler.
type Listener struct {
	psIP       string
	sendPort   int
	listenPort int
	handler    PacketHandler
}

// NewListener creates a new telemetry Listener.
//   - psIP: the PlayStation 5 IP address to send heartbeats to
//   - sendPort: the UDP port on the PS5 that receives heartbeats
//   - listenPort: the local UDP port to listen for telemetry packets
//   - handler: callback for each parsed packet
func NewListener(psIP string, sendPort, listenPort int, handler PacketHandler) *Listener {
	return &Listener{
		psIP:       psIP,
		sendPort:   sendPort,
		listenPort: listenPort,
		handler:    handler,
	}
}

// Run starts the listener. It blocks until the context is cancelled.
// It sends a heartbeat immediately and then every heartbeatInterval.
func (l *Listener) Run(ctx context.Context) error {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", l.listenPort))
	if err != nil {
		return fmt.Errorf("resolve listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}
	defer conn.Close()

	psAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", l.psIP, l.sendPort))
	if err != nil {
		return fmt.Errorf("resolve PS addr: %w", err)
	}

	// Send initial heartbeat.
	if err := l.sendHeartbeat(conn, psAddr); err != nil {
		return fmt.Errorf("initial heartbeat: %w", err)
	}

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// Channel for received packets.
	type udpMessage struct {
		data []byte
		n    int
		err  error
	}
	msgCh := make(chan udpMessage, 16)

	// Start reader goroutine.
	go func() {
		for {
			buf := make([]byte, readBufferSize)
			n, _, readErr := conn.ReadFromUDP(buf)
			select {
			case msgCh <- udpMessage{data: buf, n: n, err: readErr}:
			case <-ctx.Done():
				return
			}
			if readErr != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeatTicker.C:
			if err := l.sendHeartbeat(conn, psAddr); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		case msg := <-msgCh:
			if msg.err != nil {
				// If context was cancelled, the read error is expected.
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("read UDP: %w", msg.err)
			}
			l.handleRawPacket(msg.data[:msg.n])
		}
	}
}

// sendHeartbeat sends the heartbeat byte "A" to the PS5.
func (l *Listener) sendHeartbeat(conn *net.UDPConn, addr *net.UDPAddr) error {
	_, err := conn.WriteToUDP([]byte("A"), addr)
	return err
}

// handleRawPacket attempts to decrypt and parse a raw packet, calling the handler on success.
func (l *Listener) handleRawPacket(data []byte) {
	decrypted, err := DecryptPacket(data)
	if err != nil {
		return // silently skip invalid packets
	}

	pkt, err := ParsePacket(decrypted)
	if err != nil {
		return
	}

	l.handler(pkt)
}
