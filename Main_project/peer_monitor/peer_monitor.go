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
	for {
		select {
		case update, ok := <-peerUpdateChan:
			// If peerUpdates closes unexpectedly, the monitorPeers() function will exit and stop running
			if !ok {
				fmt.Println("peerUpdateChan closed! Restarting monitorPeers...")
				go monitorPeers(peerUpdateChan, lostPeerChan) // Restart monitoring
				return
			}

			// Handle peer updates correctly
			fmt.Printf("Received peer update: New=%v, Lost=%v\n", update.New, update.Lost)
			network.UpdateElevatorStates(update.New, update.Lost)

			for _, lostPeer := range update.Lost {
				fmt.Printf("Elevator %s disconnected!\n", lostPeer)
				lostPeerChan <- lostPeer
			}
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

