#!/usr/bin/env python3
"""UDP relay for GT7 telemetry - works around macOS firewall blocking Go binaries.

Listens on the telemetry port (33740), sends heartbeats to discover the PS5,
and forwards all received packets to the Go collector on a local relay port.
"""
import socket
import sys
import time
import struct
import threading

SEND_PORT = 33739
LISTEN_PORT = 33740
RELAY_PORT = 33741  # Collector listens here instead
HEARTBEAT_INTERVAL = 10
SCAN_INTERVAL = 30

def get_subnet_hosts():
    """Get all /24 subnet host addresses from local interfaces."""
    import fcntl
    import array
    hosts = []
    try:
        # Use netifaces-like approach via socket
        import subprocess
        result = subprocess.run(['ifconfig'], capture_output=True, text=True)
        current_ip = None
        current_mask = None
        for line in result.stdout.split('\n'):
            line = line.strip()
            if line.startswith('inet ') and '127.0.0.1' not in line:
                parts = line.split()
                ip = parts[1]
                # Find netmask
                for i, p in enumerate(parts):
                    if p == 'netmask':
                        mask_hex = parts[i+1]
                        mask_int = int(mask_hex, 16)
                        # Only /24 or smaller
                        host_bits = 32 - bin(mask_int).count('1')
                        if host_bits > 8:
                            continue
                        ip_parts = [int(x) for x in ip.split('.')]
                        ip_int = struct.unpack('!I', bytes(ip_parts))[0]
                        net_int = ip_int & mask_int
                        num_hosts = 1 << host_bits
                        for i in range(1, num_hosts - 1):
                            host_int = net_int | i
                            if host_int == ip_int:
                                continue
                            host_ip = socket.inet_ntoa(struct.pack('!I', host_int))
                            hosts.append(host_ip)
    except Exception as e:
        print(f"Error getting interfaces: {e}", file=sys.stderr)
    return hosts


def main():
    # LAN-facing socket (receives from PS5)
    lan_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    lan_sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    lan_sock.bind(('', LISTEN_PORT))
    lan_sock.settimeout(1.0)

    # Relay socket (forwards to Go collector on localhost)
    relay_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    hosts = get_subnet_hosts()
    print(f"UDP relay started: scanning {len(hosts)} hosts, forwarding to localhost:{RELAY_PORT}")

    discovered_ip = None
    last_packet_time = 0

    def scan():
        for host in hosts:
            try:
                lan_sock.sendto(b'A', (host, SEND_PORT))
            except:
                pass

    def heartbeat_loop():
        nonlocal discovered_ip, last_packet_time
        scan()  # Initial scan
        while True:
            time.sleep(HEARTBEAT_INTERVAL)
            if discovered_ip and (time.time() - last_packet_time) < SCAN_INTERVAL:
                try:
                    lan_sock.sendto(b'A', (discovered_ip, SEND_PORT))
                except:
                    pass
            else:
                if discovered_ip:
                    print(f"PS5 at {discovered_ip} stopped responding, rescanning...")
                    discovered_ip = None
                scan()

    threading.Thread(target=heartbeat_loop, daemon=True).start()

    while True:
        try:
            data, addr = lan_sock.recvfrom(2048)
            src_ip = addr[0]
            if src_ip != '127.0.0.1':
                if src_ip != discovered_ip:
                    print(f"PS5 discovered at {src_ip}")
                    discovered_ip = src_ip
                last_packet_time = time.time()
            # Forward to Go collector
            relay_sock.sendto(data, ('127.0.0.1', RELAY_PORT))
        except socket.timeout:
            continue
        except KeyboardInterrupt:
            break

    lan_sock.close()
    relay_sock.close()


if __name__ == '__main__':
    main()
