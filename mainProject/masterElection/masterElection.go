package masterElection

import (
	"mainProject/config"
	"mainProject/communication"
	"fmt"
)

// Runs Master Election and Listens for Updates
func RunMasterElection(elevatorStateChan chan map[string]communication.ElevatorStatus, masterChan chan string) {
	go func() {
		for elevatorStates := range elevatorStateChan {
			newMasterID := determineMaster(elevatorStates, config.LocalID)
			if config.MasterID == newMasterID {
				return
			}
			config.MasterID = newMasterID
			fmt.Printf("New Master Elected: %s\n\n", config.MasterID)
			masterChan <- config.MasterID
		}
	}()
}

// Assign the lowest ID as master
func determineMaster(elevatorStates map[string]communication.ElevatorStatus, localID string) string{
	lowestID := localID
	for id := range elevatorStates {
		if id < lowestID {
			lowestID = id
		}
	}
	return lowestID
}
