// In:
//		peer_monitor.go (via UpdateElevatorStates()) → Updates elevator states.
//		master_election.go (via masterChan) → Updates master ID.

// Out:
//		elevatorStateChan → (Used by order_assignment.go & master_election.go) Sends updated elevator states.
//  	bcast.Transmitter() → Broadcasts elevator states to all nodes.

package network

import (
	"Main_project/config"
	"Main_project/network/bcast"
	"Main_project/network/peers"
	"fmt"
	"sync"
	"time"
)

const (
	broadcastPort = 30000
)

// **Data structure for elevator status messages**
type ElevatorStatus struct {
	ID        string
	Floor     int
	Direction config.ElevatorState
	Queue     [config.NumFloors][config.NumButtons]bool
	Timestamp time.Time
}

// **Global map to track all known elevators**
var (
	elevatorStates = make(map[string]ElevatorStatus)
	stateMutex		sync.Mutex
	txChan			= make(chan ElevatorStatus, 10) // Global transmitter channel
)

// **Start Network: Continuously Broadcast Elevator States**
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate) {
	// Start peer reciver to get updates from other elevators
	go peers.Receiver(30001, peerUpdates)
	
	go func() {
		for {
			stateMutex.Lock()
			copyMap := make(map[string]ElevatorStatus)
			for k, v := range elevatorStates {
				copyMap[k] = v
			}
			stateMutex.Unlock()
			fmt.Println("Broadcasting elevator states...")
			elevatorStateChan <- copyMap // Send latest elevator states to all modules
			time.Sleep(100 * time.Millisecond) // Prevents excessive updates
		}
	}()

	go bcast.Transmitter(broadcastPort, txChan)
}

// **Update Elevator States from Peer Monitor**
func UpdateElevatorStates(newPeers []string, lostPeers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// **Add new peers**
	for _, newPeer := range newPeers {
		if _, exists := elevatorStates[newPeer]; !exists {
			fmt.Printf("Adding new elevator %s to state map\n", newPeer)
			elevatorStates[newPeer] = ElevatorStatus{
				ID:        newPeer,
				Timestamp: time.Now(),
			}
		}
	}

	// **Remove lost peers**
	for _, lostPeer := range lostPeers {
		fmt.Printf("Removing lost elevator %s from state map\n", lostPeer)
		delete(elevatorStates, lostPeer)
	}
}

// **Broadcast this elevator's state to the network**
func BroadcastElevatorStatus(e config.Elevator) {
	stateMutex.Lock()
	status := ElevatorStatus{
		ID:        config.LocalID,
		Floor:     e.Floor,
		Direction: e.State,
		Queue:     e.Queue,
		Timestamp: time.Now(),
	}
	stateMutex.Unlock()

	txChan <- status
}

// **Receive Elevator Status Updates from Other Elevators**
func ReceiveElevatorStatus(rxChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxChan)

	for {
		update := <-rxChan
		stateMutex.Lock()
		elevatorStates[update.ID] = update
		stateMutex.Unlock()
	}
}

