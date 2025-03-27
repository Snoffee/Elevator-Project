package masterElection

import (
	"mainProject/config"
	"mainProject/communication"
	"fmt"
	"sync"
)

var (
	stateMutex 	sync.Mutex
)

// Runs Master Election and Listens for Updates
func RunMasterElection(elevatorStateChan chan map[string]communication.ElevatorStatus, masterChan chan string) {
	go func() {
		for elevatorStates := range elevatorStateChan {
			ElectMaster(elevatorStates, masterChan)
		}
	}()
}

// Elect Master: Assign the lowest ID as master
func ElectMaster(elevatorStates map[string]communication.ElevatorStatus, masterChan chan string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	lowestID := config.LocalID
	for id := range elevatorStates {
		if id < lowestID {
			lowestID = id
		}
	}
    if config.MasterID == lowestID {
        return
    }

    config.MasterID = lowestID
    fmt.Printf("New Master Elected: %s\n\n", config.MasterID)
    masterChan <- config.MasterID
}
