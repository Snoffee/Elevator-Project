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
	"time"
)

var (
	stateMutex 	sync.Mutex
	masterID   	string
	masterVersion int
)

// **Runs Master Election and Listens for Updates**
func RunMasterElection(elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string, heartbeatChan chan string) {
	go func() {
		for elevatorStates := range elevatorStateChan {
			electMaster(elevatorStates, masterChan)
		}
	}()
	go monitorHeartbeats(masterChan, heartbeatChan)
	go ReceiveMasterUpdates(masterChan)
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
		masterVersion++ // Increment masterVersion
		fmt.Printf("New Master Elected: %s\n", masterID)
		masterChan <- masterID // Notify system
	}
}

// **Monitor Heartbeats to Detect If Master Is Alive**
func monitorHeartbeats(masterChan chan string, heartbeatChan chan string) {
	for {
		time.Sleep(500 * time.Millisecond)

		stateMutex.Lock()
		currentMaster := masterID
		stateMutex.Unlock()

		select {
		case heartbeat := <-heartbeatChan:
			// If we receive a heartbeat from the current master, it's alive
			if heartbeat == currentMaster {
				continue
			}
		default:
			// If no heartbeat received, trigger re-election
			fmt.Println("Master not responding, triggering re-election...")
			masterChan <- ""
		}
	}
}

// **Receive Master Updates and Prevent Split-Brain**
func ReceiveMasterUpdates(masterChan chan string) {
	go func() {
		for newMaster := range masterChan {
			stateMutex.Lock()

			// If we get an empty master signal, force re-election
			if newMaster == "" {
				fmt.Println("Forcing Master Re-Election...")
				masterVersion++ // Increase version to prevent old master from reclaiming
				stateMutex.Unlock()
				continue
			}

			// Compare versions before updating master
			if masterVersion > 0 && masterID != "" {
				fmt.Printf("Comparing Versions: Current=%d, New=%d\n", masterVersion, masterVersion+1)
				if masterVersion >= masterVersion+1 {
					fmt.Println("Ignoring older master version...")
					stateMutex.Unlock()
					continue
				}
			}

			masterID = newMaster
			masterVersion++ // Ensure version increases
			fmt.Printf("Updated masterID: %s (Version %d)\n", newMaster, masterVersion)

			stateMutex.Unlock()
		}
	}()
}