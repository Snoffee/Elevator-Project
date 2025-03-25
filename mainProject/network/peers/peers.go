package peers

import (
	"mainProject/network/conn"
	"fmt"
	"net"
	"sort"
	"time"
)

type PeerUpdate struct {
	Peers []string
	New   []string
	Lost  []string
}

const interval = 15 * time.Millisecond
const timeout = 2000 * time.Millisecond
const maxMissedHeartbeats = 3                  // Require 3 consecutive misses before marking peer as lost

func Transmitter(port int, id string, transmitEnable <-chan bool) {

	conn := conn.DialBroadcastUDP(port)
	addr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", port))

	enable := true
	for {
		select {
		case enable = <-transmitEnable:
		case <-time.After(interval):
		}
		if enable {
			conn.WriteTo([]byte(id), addr)
		}
	}
}

func Receiver(port int, peerUpdateCh chan<- PeerUpdate) {

	var buf [1024]byte
	var p PeerUpdate
	lastSeen := make(map[string]time.Time)
	missedHeartbeats := make(map[string]int)    // Track missed heartbeats per peer

	conn := conn.DialBroadcastUDP(port)

	for {
		updated := false

		conn.SetReadDeadline(time.Now().Add(interval))
		n, _, _ := conn.ReadFrom(buf[0:])

		id := string(buf[:n])

		// Adding new connection
		p.New = []string{}
		if id != "" {
			if _, idExists := lastSeen[id]; !idExists {
				p.New = append(p.New, id)
				updated = true
			}

			lastSeen[id] = time.Now()
			missedHeartbeats[id] = 0            // Reset missed heartbeat count when a message is received

		}

		// Removing dead connection
		p.Lost = make([]string, 0)
		for k, lastTime := range lastSeen {
			if time.Since(lastTime) > timeout {
				missedHeartbeats[k]++
                if missedHeartbeats[k] >= maxMissedHeartbeats {
					updated = true
					p.Lost = append(p.Lost, k)
					delete(lastSeen, k)
					delete(missedHeartbeats, k)
				}
			}
		}

		// Sending update
		if updated {
			p.Peers = make([]string, 0, len(lastSeen))

			for k := range lastSeen {
				p.Peers = append(p.Peers, k)
			}

			sort.Strings(p.Peers)
			sort.Strings(p.Lost)
			peerUpdateCh <- p
		}
	}
}
