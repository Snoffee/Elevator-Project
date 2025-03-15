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
	assignmentPort  = 30002 // Port for broadcasting assigned hall calls
	rawHallCallPort = 30003 // Port for raw hall calls (hall calls received by slaves, that needs to be forwarded to the master before assigning them)
	statusPort = 30004 // Port for hall call confirmations
	lightPort = 30005 // Port for light orders (hall call lights)
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

type OrderStatus int

const (
	Unfinished   OrderStatus = 0
	Finished                 = 1
)

type OrderStatusMessage struct {
    SenderID   string
    ButtonEvent elevio.ButtonEvent
	Status     OrderStatus
}

type LightStatus int

const (
	Off   LightStatus = 0
	On                = 1
)

type LightOrderMessage struct {
	TargetID    string
	ButtonEvent elevio.ButtonEvent
	Light      LightStatus
}


var (
	elevatorStates    	 = make(map[string]ElevatorStatus) // Global map to track all known elevators
	backupElevatorStates = make(map[string]ElevatorStatus)
	stateMutex			 sync.Mutex
	txElevatorStatusChan = make(chan ElevatorStatus, 10) // Global transmitter channel
	rxElevatorStatusChan = make(chan ElevatorStatus, 10) // Global receiver channel
	txAssignmentChan  	 = make(chan AssignmentMessage, 10) // Global channel for assignments
	txRawHallCallChan	 = make(chan RawHallCallMessage, 10) // Raw hall call events
	txLightChan	 = make(chan LightOrderMessage, 20) // transmit light orders
	rxOrderStatusChan = make(chan OrderStatusMessage, 10) // Receive confirmation of hall calls
	txOrderStatusChan = make(chan OrderStatusMessage, 10) // Transmit confirmation of hall calls
)

// **Start Network: Continuously Broadcast Elevator States**
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate, orderStatusChan chan OrderStatusMessage) {
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

	// Start broadcasting assignments
	go bcast.Transmitter(assignmentPort, txAssignmentChan)

	// Start broadcasting raw hall calls
	go bcast.Transmitter(rawHallCallPort, txRawHallCallChan)

	// Start broadcasting hall call status
	go bcast.Transmitter(statusPort, txOrderStatusChan)

	// Start receiving hall call status
	go bcast.Receiver(statusPort, rxOrderStatusChan)

	go func() {
        for {
            msg := <-rxOrderStatusChan
            orderStatusChan <- msg
        }
    }()

	// Start broadcasting light orders
	go bcast.Transmitter(lightPort, txLightChan)

}

// **Updates the global elevator state when a new peer joins or an elevator disconnects**
func UpdateElevatorStates(newPeers []string, lostPeers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// Ensure the backup keeps all previously lost elevators
	for _, lostPeer := range lostPeers {
		if _, exists := elevatorStates[lostPeer]; exists {
			fmt.Printf("Backing up lost elevator: %s\n\n", lostPeer)
			backupElevatorStates[lostPeer] = elevatorStates[lostPeer]
		}
	}

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
		fmt.Printf("Removing lost elevator %s from state map\n\n", lostPeer)
		delete(elevatorStates, lostPeer)
	}
}

// **Retrieve backup state for cab call reassignment**
func GetBackupState() map[string]ElevatorStatus {
	return backupElevatorStates
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
func ReceiveElevatorStatus(rxElevatorStatusChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxElevatorStatusChan)

	for {
		hallAssignment := <-rxElevatorStatusChan

		stateMutex.Lock()
		elevatorStates[hallAssignment.ID] = hallAssignment
		stateMutex.Unlock()
	}
}

// **Broadcasts assigned assignments over the network**
func SendAssignment(targetElevator string, floor int, button elevio.ButtonType) {
	fmt.Printf("Sending assignment to %s for floor %d\n", targetElevator, floor)

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

func SendOrderStatus(msg OrderStatusMessage) {
    txOrderStatusChan <- msg
}


func SendLightOrder(buttonLight elevio.ButtonEvent, lightOnOrOff LightStatus) {
	stateMutex.Lock()
    defer stateMutex.Unlock()
    // Create tagged light order messages
	for _, elevator := range elevatorStates {
		if elevator.ID == config.LocalID {
			continue
		}
    	msg := LightOrderMessage{
        	TargetID: elevator.ID,
        	ButtonEvent: buttonLight,
			Light: lightOnOrOff,
    	}
    txLightChan <- msg
	}
}

