// In:
//	elevatorStateChan (from network.go) → Receives the latest elevator states to decide the master.
//
// Out:
//	masterChan (used by network.go & order_assignment.go) → Notifies all modules when a new master is elected.

package masterElection

import (
	"mainProject/config"
	"mainProject/network"
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
			ElectMaster(elevatorStates, masterChan)
		}
	}()
}

// **Elect Master: Assign the lowest ID as master**
func ElectMaster(elevatorStates map[string]network.ElevatorStatus, masterChan chan string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Find the lowest ID among active elevators
	lowestID := config.LocalID
	for id := range elevatorStates {
		if id < lowestID {
			lowestID = id
		}
	}

    if masterID == lowestID {
        return
    }

    masterID = lowestID
    masterVersion++
    fmt.Printf("New Master Elected: %s (Version %d)\n\n", masterID, masterVersion)
    masterChan <- masterID
}
