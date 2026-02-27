package telemetry

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	heartbeatInterval = 10 * time.Second
	readBufferSize    = 2048
	// discoveryStaleTimeout is how long without packets before rescanning.
	discoveryStaleTimeout = 30 * time.Second
)

// PacketHandler is a callback invoked for each successfully decrypted and parsed packet.
type PacketHandler func(*Packet)

// Listener receives GT7 telemetry data via UDP.
// It sends periodic heartbeat messages to the PS5 and listens for encrypted
// telemetry packets, decrypting and parsing them before passing to the handler.
// If psIP is empty, it scans the local subnet to find the PS5 automatically.
type Listener struct {
	psIP       string
	sendPort   int
	listenPort int
	handler    PacketHandler
}

// NewListener creates a new telemetry Listener.
//   - psIP: the PlayStation 5 IP address (empty for subnet scan auto-discovery)
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

type udpMessage struct {
	data []byte
	n    int
	addr *net.UDPAddr
	err  error
}

// Run starts the listener. It blocks until the context is cancelled.
func (l *Listener) Run(ctx context.Context) error {
	listenAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", l.listenPort))
	if err != nil {
		return fmt.Errorf("resolve listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}
	defer conn.Close()

	// Start reader goroutine.
	msgCh := make(chan udpMessage, 256)
	go func() {
		log.Printf("Reader goroutine started, waiting for packets on :%d", l.listenPort)
		for {
			buf := make([]byte, readBufferSize)
			n, addr, readErr := conn.ReadFromUDP(buf)
			select {
			case msgCh <- udpMessage{data: buf, n: n, addr: addr, err: readErr}:
			case <-ctx.Done():
				return
			}
			if readErr != nil {
				log.Printf("Reader goroutine error: %v", readErr)
				return
			}
		}
	}()

	if l.psIP != "" {
		return l.runStatic(ctx, conn, msgCh)
	}
	return l.runDiscovery(ctx, conn, msgCh)
}

// runStatic sends heartbeats to a fixed PS5 IP.
func (l *Listener) runStatic(ctx context.Context, conn *net.UDPConn, msgCh <-chan udpMessage) error {
	psAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", l.psIP, l.sendPort))
	if err != nil {
		return fmt.Errorf("resolve PS addr: %w", err)
	}

	if err := l.sendHeartbeat(conn, psAddr); err != nil {
		return fmt.Errorf("initial heartbeat: %w", err)
	}

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

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
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("read UDP: %w", msg.err)
			}
			l.handleRawPacket(msg.data[:msg.n])
		}
	}
}

// runDiscovery scans the local subnet to find the PS5, then sends targeted heartbeats.
// If the PS5 stops responding, it rescans automatically.
func (l *Listener) runDiscovery(ctx context.Context, conn *net.UDPConn, msgCh <-chan udpMessage) error {
	scanTargets := subnetAddrs(l.sendPort)
	if len(scanTargets) == 0 {
		return fmt.Errorf("auto-discovery: no local network interfaces found")
	}
	log.Printf("Auto-discovery: scanning %d addresses on local subnet(s)", len(scanTargets))

	var discoveredIP string
	var lastPacketTime time.Time

	// Initial scan.
	l.scanSubnet(conn, scanTargets)

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-heartbeatTicker.C:
			if discoveredIP != "" && time.Since(lastPacketTime) < discoveryStaleTimeout {
				// PS5 is known and responding — send targeted heartbeat.
				addr, addrErr := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", discoveredIP, l.sendPort))
				if addrErr != nil {
					log.Printf("Failed to resolve PS5 addr %s: %v", discoveredIP, addrErr)
				} else if err := l.sendHeartbeat(conn, addr); err != nil {
					log.Printf("Heartbeat to %s failed: %v", discoveredIP, err)
				}
			} else {
				// PS5 unknown or went stale — rescan.
				if discoveredIP != "" {
					log.Printf("PS5 at %s stopped responding, rescanning...", discoveredIP)
					discoveredIP = ""
				}
				l.scanSubnet(conn, scanTargets)
			}

		case msg := <-msgCh:
			if msg.err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("read UDP: %w", msg.err)
			}

			// Track the source IP of incoming packets.
			if msg.addr != nil {
				srcIP := msg.addr.IP.String()
				if srcIP != discoveredIP {
					log.Printf("PS5 discovered at %s", srcIP)
					discoveredIP = srcIP
				}
				lastPacketTime = time.Now()
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

// scanSubnet sends a heartbeat to every address in the list.
func (l *Listener) scanSubnet(conn *net.UDPConn, addrs []*net.UDPAddr) {
	for _, addr := range addrs {
		conn.WriteToUDP([]byte("A"), addr)
	}
}

// handleRawPacket attempts to decrypt and parse a raw packet, calling the handler on success.
func (l *Listener) handleRawPacket(data []byte) {
	if len(data) < PacketSize {
		return // Ignore undersized packets (e.g. heartbeat echoes).
	}
	decrypted, err := DecryptPacket(data)
	if err != nil {
		log.Printf("Decrypt failed (%d bytes): %v", len(data), err)
		return
	}

	pkt, err := ParsePacket(decrypted)
	if err != nil {
		log.Printf("Parse failed: %v", err)
		return
	}

	if l.handler != nil {
		l.handler(pkt)
	}
}

// subnetAddrs returns UDP addresses for all hosts on local /24 (or smaller) subnets.
func subnetAddrs(port int) []*net.UDPAddr {
	var addrs []*net.UDPAddr
	ifaces, err := net.Interfaces()
	if err != nil {
		return addrs
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces.
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		ifAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range ifAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil {
				continue
			}

			ones, bits := ipNet.Mask.Size()
			hostBits := bits - ones
			// Only scan subnets up to /24 (254 hosts). Larger subnets
			// would take too long; set the PS5 IP manually instead.
			if hostBits > 8 {
				continue
			}

			networkInt := binary.BigEndian.Uint32(ip) & binary.BigEndian.Uint32(ipNet.Mask)
			myIP := binary.BigEndian.Uint32(ip)
			numHosts := uint32(1) << hostBits

			for i := uint32(1); i < numHosts-1; i++ {
				hostInt := networkInt | i
				if hostInt == myIP {
					continue
				}
				hostIP := make(net.IP, 4)
				binary.BigEndian.PutUint32(hostIP, hostInt)
				addrs = append(addrs, &net.UDPAddr{IP: hostIP, Port: port})
			}
		}
	}

	return addrs
}
