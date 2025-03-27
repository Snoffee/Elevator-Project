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
)

// Runs Master Election and Listens for Updates
func RunMasterElection(elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string) {
	go func() {
		for elevatorStates := range elevatorStateChan {
			ElectMaster(elevatorStates, masterChan)
		}
	}()
}

// Elect Master: Assign the lowest ID as master
func ElectMaster(elevatorStates map[string]network.ElevatorStatus, masterChan chan string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

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
    fmt.Printf("New Master Elected: %s\n\n", masterID)
    masterChan <- masterID
}
