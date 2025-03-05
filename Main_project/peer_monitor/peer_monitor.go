// In:
// peers.PeerUpdate (tracks peer changes from the network).

//Out:
// network.go (via UpdateElevatorStates()) → Updates global elevator states.
// order_assignment.go (via lostPeerChan) → Triggers order reassignment when an elevator disconnects.

package peer_monitor

import (
	"Main_project/config"
	"Main_project/network"
	"Main_project/network/peers"
	"time"
	"fmt"
)

// **Runs MonitorPeers as a Goroutine**
func RunMonitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string) {
	go monitorPeers(peerUpdateChan, lostPeerChan)
	go announceSelf()
}

// **Monitor Peers and Notify Master Election & Order Assignment**
func monitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string) {
	for update := range peerUpdateChan {
		// Send peer updates to network.go
		network.UpdateElevatorStates(update.New, update.Lost)

		// Notify Order Assignment of lost elevators
		for _, lostPeer := range update.Lost {
			fmt.Printf("Elevator %s disconnected!\n", lostPeer)
			lostPeerChan <- lostPeer
		}
	}
}


// **Announce Self to the Network**
func announceSelf() {
	txEnable := make(chan bool, 1)
	txEnable <- true

	go peers.Transmitter(30001, config.LocalID, txEnable) // Sends ID updates

	for {
		time.Sleep(1 * time.Second) // Keeps sending updates
	}
}

