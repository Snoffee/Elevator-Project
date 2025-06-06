package peers

import (
	"fmt"
	"mainProject/network/conn"
	"net"
	"sort"
	"time"
	"mainProject/config"
)

type PeerUpdate struct {
	Peers []string
	New   []string
	Lost  []string
}

const interval = 15 * time.Millisecond
const timeout = 2000 * time.Millisecond

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
			for i := 0; i < 3; i++ {
				conn.WriteTo([]byte(id), addr)
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}

func Receiver(port int, peerUpdateCh chan<- PeerUpdate) {

	var buf [1024]byte
	var p PeerUpdate
	lastSeen := make(map[string]time.Time)

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
		}

		// Removing dead connection
		p.Lost = make([]string, 0)
		for peer, lastTime := range lastSeen {
			if time.Since(lastTime) > timeout && peer != config.LocalID{ // An elevator should not remove itself from the network
				updated = true
				p.Lost = append(p.Lost, peer)
				delete(lastSeen, peer)
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
