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
	peerPort      = 30001 // Port for receiving elevator state updates
	hallCallPort  = 30002 // Port for broadcasting assigned hall calls
	rawHallCallPort = 30003 // Port for raw hall calls (hall calls received by slaves, that needs to be forwarded to the master before assigning them)
)

// **Data structure for elevator status messages**
type ElevatorStatus struct {
	ID        string
	Floor     int
	Direction config.ElevatorState
	Queue     [config.NumFloors][config.NumButtons]bool
	Timestamp time.Time
}

type AssignmentMessage struct {
	TargetID string
	Floor    int
	Button   elevio.ButtonType
}

type RawHallCallMessage struct {
    TargetID string
    Floor    int
    Button   elevio.ButtonType
}

var (
	elevatorStates    	 = make(map[string]ElevatorStatus) // Global map to track all known elevators
	stateMutex			 sync.Mutex
	txElevatorStatusChan = make(chan ElevatorStatus, 10) // Global transmitter channel
	rxElevatorStatusChan = make(chan ElevatorStatus, 10) // Global receiver channel
	txAssignmentChan  	 = make(chan AssignmentMessage, 10) // Global channel for hall assignments
	txRawHallCallChan	 = make(chan RawHallCallMessage, 10) // Raw hall call events
)

// **Start Network: Continuously Broadcast Elevator States**
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate) {
	// Start peer reciver to get updates from other elevators
	go peers.Receiver(peerPort, peerUpdates)

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
	go bcast.Transmitter(broadcastPort, txElevatorStatusChan)

	// Start elevator status receiver
	go ReceiveElevatorStatus(rxElevatorStatusChan)

	// Start broadcasting hall calls
	go bcast.Transmitter(hallCallPort, txAssignmentChan)

	// Start broadcasting raw hall calls
	go bcast.Transmitter(rawHallCallPort, txRawHallCallChan)

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

	txElevatorStatusChan <- status
}

// **Receives and updates elevator status messages from other elevators**
func ReceiveElevatorStatus(rxChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxChan)

	for {
		hallAssignment := <-rxChan

		stateMutex.Lock()
		elevatorStates[hallAssignment.ID] = hallAssignment
		stateMutex.Unlock()
	}
}

// **Broadcast cab assignment over the network**
func SendCabAssignment(targetElevator string, floor int) {
	fmt.Printf("Sending cab assignment to %s for floor %d\n", targetElevator, floor)

	cabCall := AssignmentMessage{
		TargetID: targetElevator,
		Floor:    floor,
		Button:   elevio.BT_Cab, 
	}

	txAssignmentChan <- cabCall
}

// **Broadcasts assigned hall calls over the network**
// Send hall assignment to a specific elevator
func SendHallAssignment(targetElevator string, floor int, button elevio.ButtonType) {
	fmt.Printf("Sending hall assignment to %s for floor %d\n", targetElevator, floor)

	hallCall := AssignmentMessage{
		TargetID: targetElevator,
		Floor:    floor,
		Button:   button,
	}

	txAssignmentChan <- hallCall
}

// **SendRawHallCall sends a raw hall call event over the network**
// hall calls received by slaves need to be broadcasted to master for assignment
func SendRawHallCall(masterID string, hallCall elevio.ButtonEvent) {
    if config.LocalID == masterID {
        // This node is the master – no need to forward the call
        return
    }
    // Send hall call directly to the current master
    msg := RawHallCallMessage{TargetID: masterID, Floor: hallCall.Floor, Button: hallCall.Button}
    txRawHallCallChan <- msg

    // Start a brief timeout to retry if master doesn’t respond quickly
    go func(call RawHallCallMessage) {
        time.Sleep(100 * time.Millisecond)          // small delay before retry
        txRawHallCallChan <- call                   // resend the hall call to master
    }(msg)
}