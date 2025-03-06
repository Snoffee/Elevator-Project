// In:
//		peer_monitor.go (via UpdateElevatorStates()) → Updates the global elevator state map.
//		master_election.go (via masterChan) → Updates the master ID.
//		single_elevator.go (via BroadcastElevatorStatus()) → Sends individual elevator status updates.

// Out:
//		elevatorStateChan → (Used by order_assignment.go & master_election.go) Sends the latest global elevator states.
//  	bcast.Transmitter() → Broadcasts elevator states to all nodes via UDP.
//  	BroadcastHallAssignment() → Sends assigned hall calls over the network to all elevators.

package network

import (
	"Main_project/config"
	"Main_project/network/bcast"
	"Main_project/network/peers"
	"Main_project/elevio"
	"fmt"
	"sync"
	"time"
)

const (
	broadcastPort = 30000 // Port for broadcasting elevator states
	hallCallPort  = 30002 // Port for broadcasting assigned hall calls
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

	// Periodically send updated elevator states to other modules
	go func() {
		for {
			stateMutex.Lock()
			copyMap := make(map[string]ElevatorStatus)
			for k, v := range elevatorStates {
				copyMap[k] = v
			}
			stateMutex.Unlock()
			elevatorStateChan <- copyMap // Send latest elevator states to all modules
			time.Sleep(100 * time.Millisecond) // Prevents excessive updates
		}
	}()

	// Start broadcasting elevator states
	go bcast.Transmitter(broadcastPort, txChan)
}

// **Updates the global elevator state when a new peer joins or an elevator disconnects**
func UpdateElevatorStates(newPeers []string, lostPeers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Add new elevators to the state map
	for _, newPeer := range newPeers {
		if _, exists := elevatorStates[newPeer]; !exists {
			fmt.Printf("Adding new elevator %s to state map\n", newPeer)
			elevatorStates[newPeer] = ElevatorStatus{
				ID:        newPeer,
				Timestamp: time.Now(),
			}
		}
	}

	// Remove lost elevators from the state map
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

// **Receives and updates elevator status messages from other elevators**
func ReceiveElevatorStatus(rxChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxChan)

	for {
		update := <-rxChan
		stateMutex.Lock()
		elevatorStates[update.ID] = update
		stateMutex.Unlock()
	}
}

// **Broadcasts assigned hall calls over the network**
func BroadcastHallAssignment(elevatorID string, hallCall elevio.ButtonEvent) {
	txChan := make(chan elevio.ButtonEvent, 10) 
	go bcast.Transmitter(hallCallPort, txChan)

	txChan <- hallCall // Send the assigned hall call to all elevators
}

