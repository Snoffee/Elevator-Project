package communication

import (
	"mainProject/config"
	"mainProject/network/bcast"
	"time"
)

// Elevator State Management
// -----------------------------------------------------------------------------
// Updates the global elevatorStatuses map when new elevators join or existing elevators disconnect.
// Backs up the state of lost elevators for potential reassignment of cab calls.
func UpdateElevatorStates(newPeers []string, lostPeers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	for _, lostPeer := range lostPeers {
		if _, exists := elevatorStatuses[lostPeer]; exists {
			backupElevatorStatuses[lostPeer] = elevatorStatuses[lostPeer]
		}
	}
	for _, newPeer := range newPeers {
		if _, exists := elevatorStatuses[newPeer]; !exists {
			elevatorStatuses[newPeer] = ElevatorStatus{
				ID:        newPeer,
				Timestamp: time.Now(),
			}
		}
	}
	for _, lostPeer := range lostPeers {
		delete(elevatorStatuses, lostPeer)
	}
}

// Returns the backup state of lost elevators for cab call reassignment.
func GetBackupState() map[string]ElevatorStatus {
	return backupElevatorStatuses
}

// Sends a copy of the current status map to the elevatorStatusesChan for internal use (e.g., order assignment, master election)
func startPeriodicLocalStatusUpdates(elevatorStatusesChan chan map[string]ElevatorStatus) {
    go func() {
        for {
            stateMutex.Lock()
            copyMap := make(map[string]ElevatorStatus)
            for k, v := range elevatorStatuses {
                copyMap[k] = v
            }
            stateMutex.Unlock()
            elevatorStatusesChan <- copyMap 
            time.Sleep(500 * time.Millisecond) 
        }
    }()
}

// Broadcasts local elevator state to other elevators and updates the global elevatorStatuses map
// Sends immediate status updates when critical events happen (e.g., a floor is reached, a hall call is assigned).
func BroadcastElevatorStatus(e config.Elevator, isCriticalEvent bool) {
    stateMutex.Lock()
    localElevatorStatus := ElevatorStatus{
        ID:        config.LocalID,
        Floor:     e.Floor,
		State: e.State,
        Direction: e.Direction,
        Queue:     e.Queue,
        Timestamp: time.Now(),
    }
    elevatorStatuses[config.LocalID] = localElevatorStatus  // Always update the local map
    stateMutex.Unlock()

    redundancyFactor := 3  // For periodic broadcasts
    if isCriticalEvent {
        redundancyFactor = 10  // Increase redundancy for critical events
    }

    for i := 0; i < redundancyFactor; i++ {
        txElevatorStatusChan <- localElevatorStatus
        time.Sleep(5 * time.Millisecond)
    }
}

// Receives elevator state updates from other elevators and updates the global elevatorStatuses map.
func ReceiveElevatorStatus(rxElevatorStatusChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxElevatorStatusChan)

	for {
		hallAssignment := <-rxElevatorStatusChan

		stateMutex.Lock()
		elevatorStatuses[hallAssignment.ID] = hallAssignment
		stateMutex.Unlock()
	}
}
