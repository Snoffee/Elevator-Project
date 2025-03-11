// In:
//	elevatorStateChan (from network.go) → Receives the latest elevator states to decide the master.
//
// Out:
//	masterChan (used by network.go & order_assignment.go) → Notifies all modules when a new master is elected.

package master_election

import (
	"Main_project/config"
	"Main_project/network"
	"fmt"
	"sync"
)

var (
	stateMutex 	sync.Mutex
	masterID   	string
	masterVersion int
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

	// Find the lowest ID among active elevators
	lowestID := config.LocalID
	for id := range elevatorStates {
		if id < lowestID {
			lowestID = id
		}
	}

	// Avoid redundant re-election if the master remains unchanged
    if masterID == lowestID {
        return
    }

	// Update and notify of new master
    masterID = lowestID
    masterVersion++
    fmt.Printf("New Master Elected: %s (Version %d)\n\n", masterID, masterVersion)
    masterChan <- masterID
}
