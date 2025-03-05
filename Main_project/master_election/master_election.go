// In:
//		elevatorStateChan (from network.go) → Decides the master.

// Out:
//		masterChan (used by network.go & order_assignment.go) → Notifies new master.

package master_election

import (
	"Main_project/config"
	"Main_project/network"
	"fmt"
	"sync"
)

var (
	stateMutex sync.Mutex
	masterID   string
)

// **Runs Master Election and Listens for Updates**
func RunMasterElection(elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string) {
	go func() {
		for elevatorStates := range elevatorStateChan {
			electMaster(elevatorStates, masterChan)
		}
	}()
}

// **Elect Master: Assign the lowest ID as master**
func electMaster(elevatorStates map[string]network.ElevatorStatus, masterChan chan string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Find lowest ID from the elevator states
	lowestID := config.LocalID
	for id := range elevatorStates {
		if id < lowestID {
			lowestID = id
		}
	}

	// If master has changed, notify system
	if masterID != lowestID {
		masterID = lowestID
		fmt.Printf("New Master Elected: %s\n", masterID)
		masterChan <- masterID // Notify system
	}
}
