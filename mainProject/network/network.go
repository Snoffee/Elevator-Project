package network

import (
	"mainProject/config"
	"mainProject/network/bcast"
	"mainProject/network/peers"
	"mainProject/elevio"
	"fmt"
	"sync"
	"time"
)

const (
	broadcastPort     = 30000 // Port for broadcasting elevator states
	peerPort          = 30001 // Port for receiving elevator state updates
	assignmentPort    = 30002 // Port for broadcasting assigned hall calls
	txrawHallCallPort = 30003 // Port for raw hall calls (hall calls received by slaves, that needs to be forwarded to the master before assigning them)
	ackPort			  = 30004 // Port for reading the masters ack for hall calls from slaves
	statusPort        = 30005 // Port for hall call confirmations
	lightPort         = 30006 // Port for light orders (hall call lights)
)

// -----------------------------------------------------------------------------
// Data Structures
// -----------------------------------------------------------------------------
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
	SeqNum   int 
}

type RawHallCallMessage struct {
  	TargetID string
	SenderID string
    Floor    int
    Button   elevio.ButtonType
	SeqNum   int 
}

type AckMessage struct {
	TargetID string
	SeqNum 	 int
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
	SeqNum   int
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
	SeqNum   int
}

// -----------------------------------------------------------------------------
// Global Variables
// -----------------------------------------------------------------------------
var (
	elevatorStatuses        = make(map[string]ElevatorStatus) // Global map to track all known elevators
	backupElevatorStatuses  = make(map[string]ElevatorStatus)
	
	txElevatorStatusChan    = make(chan ElevatorStatus, 10) // Global transmitter channel
	rxElevatorStatusChan    = make(chan ElevatorStatus, 10) // Global receiver channel
	txAssignmentChan        = make(chan AssignmentMessage, 10) // Global channel for assignments
	txRawHallCallChan       = make(chan RawHallCallMessage, 10) // Slaves send hall call events to master
	txLightChan	            = make(chan LightOrderMessage, 20) // transmit light orders
	rxOrderStatusChan       = make(chan OrderStatusMessage, 10) // Receive confirmation of hall calls
	txOrderStatusChan       = make(chan OrderStatusMessage, 10) // Transmit confirmation of hall calls
	rxAckChan				= make(chan AckMessage, 10)
	txAckChan				= make(chan AckMessage, 10)
	
	resendTimeout			= 3 * time.Second
	giveUpTimeout			= 10 * time.Second

	seqNumAssignmentCounter = 0
	seqNumRawCallCounter	= 100
	SeqOrderStatusCounter   = 200
	seqLightCounter         = 300

	stateMutex	              sync.Mutex
)

// -----------------------------------------------------------------------------
// Initialization and Network Management
// -----------------------------------------------------------------------------
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate, orderStatusChan chan OrderStatusMessage) {
	// Start peer reciver to get updates from other elevators
	go peers.Receiver(peerPort, peerUpdates)

	// Periodically send updated elevator states to other modules
	startPeriodicStateUpdates(elevatorStateChan)
	
	// Start broadcasting elevator states
	go bcast.Transmitter(broadcastPort, txElevatorStatusChan)

	// Start elevator status receiver
	go ReceiveElevatorStatus(rxElevatorStatusChan)

	// Start broadcasting assignments
	go bcast.Transmitter(assignmentPort, txAssignmentChan)

	// Start broadcasting raw hall calls
	go bcast.Transmitter(txrawHallCallPort, txRawHallCallChan)

	// Start receiving and transmitting acks
	go bcast.Receiver(ackPort, rxAckChan)
	go bcast.Transmitter(ackPort, txAckChan)
	
	// Hall call status
	go bcast.Transmitter(statusPort, txOrderStatusChan)
	startReceivingHallCallStatus(orderStatusChan)
	
	// Start broadcasting light orders
	go bcast.Transmitter(lightPort, txLightChan)
}

// -----------------------------------------------------------------------------
// Elevator State Management
// -----------------------------------------------------------------------------
// Updates the global elevatorStates map when new elevators join or existing elevators disconnect.
// Backs up the state of lost elevators for potential reassignment of cab calls.
func UpdateElevatorStates(newPeers []string, lostPeers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	for _, lostPeer := range lostPeers {
		if _, exists := elevatorStatuses[lostPeer]; exists {
			fmt.Printf("Backing up lost elevator: %s\n\n", lostPeer)
			backupElevatorStatuses[lostPeer] = elevatorStatuses[lostPeer]
		}
	}
	for _, newPeer := range newPeers {
		if _, exists := elevatorStatuses[newPeer]; !exists {
			fmt.Printf("Adding new elevator %s to state map\n", newPeer)
			elevatorStatuses[newPeer] = ElevatorStatus{
				ID:        newPeer,
				Timestamp: time.Now(),
			}
		}
	}
	for _, lostPeer := range lostPeers {
		fmt.Printf("Removing lost elevator %s from state map\n\n", lostPeer)
		delete(elevatorStatuses, lostPeer)
	}
}

// Returns the backup state of lost elevators for cab call reassignment.
func GetBackupState() map[string]ElevatorStatus {
	return backupElevatorStatuses
}

// Ensures that all modules periodically receive the latest state of all elevators.
// Maintaining a consistent view of the system across all nodes, even if no immediate events occur.
func startPeriodicStateUpdates(elevatorStatusesChan chan map[string]ElevatorStatus) {
    go func() {
        for {
            stateMutex.Lock()
            copyMap := make(map[string]ElevatorStatus)
            for k, v := range elevatorStatuses {
                copyMap[k] = v
            }
            stateMutex.Unlock()
            elevatorStatusesChan <- copyMap 
            time.Sleep(100 * time.Millisecond) 
        }
    }()
}

// Sends immediate updates when critical events happen (e.g., a floor is reached, a hall call is assigned).
// Ensures that other modules or elevators are notified of state changes without waiting for the next periodic update.
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

// Receives elevator state updates from other elevators and updates the global elevatorStates map.
func ReceiveElevatorStatus(rxElevatorStatusChan chan ElevatorStatus) {
	go bcast.Receiver(broadcastPort, rxElevatorStatusChan)

	for {
		hallAssignment := <-rxElevatorStatusChan

		stateMutex.Lock()
		elevatorStatuses[hallAssignment.ID] = hallAssignment
		stateMutex.Unlock()
	}
}

// -----------------------------------------------------------------------------
// Assignment and Hall Call Management
// -----------------------------------------------------------------------------
// Sends an assignment message to a specific elevator for a hall call.
func SendAssignment(targetElevator string, floor int, button elevio.ButtonType) {	
	seqNumAssignmentCounter++
	hallCall := AssignmentMessage{
		TargetID: targetElevator,
		Floor:    floor,
		Button:   button,
		SeqNum: seqNumAssignmentCounter,
	}
	timeout := time.After(giveUpTimeout)
	for{
		select{
		case ack := <- rxAckChan:
			if ack.TargetID == targetElevator && ack.SeqNum == hallCall.SeqNum {
				fmt.Printf("Received ack from: %s | seqNum: %d\n", targetElevator, ack.SeqNum)
				return
			}
		case <-timeout:
			fmt.Println("Timeout reached, stopping attempts")
			return
		default:
			fmt.Printf("Sending assignment to %s for floor %d\n", targetElevator, hallCall.Floor)
			txAssignmentChan <- hallCall
			time.Sleep(resendTimeout)
		}
	}
}

// Sends a raw hall call event to the master elevator for assignment.
func SendRawHallCall(masterID string, hallCall elevio.ButtonEvent) {
    if config.LocalID == masterID {
        return
    }
    seqNumRawCallCounter++
    msg := RawHallCallMessage{TargetID: masterID, SenderID: config.LocalID, Floor: hallCall.Floor, Button: hallCall.Button, SeqNum: seqNumRawCallCounter}
	timeout := time.After(giveUpTimeout)
	for{
		select{
		case ack := <- rxAckChan:
			if ack.TargetID == config.LocalID && ack.SeqNum == msg.SeqNum {
				fmt.Println("Received ack from master", masterID)
				return
			}
		case <-timeout:
			fmt.Println("Timeout reached, stopping attempts")
			return
		default:
			fmt.Printf("Sending RawHallCall to master %s for floor %d\n", masterID, hallCall.Floor)
			txRawHallCallChan <- msg
			time.Sleep(resendTimeout)
		}
	}
}

// -----------------------------------------------------------------------------
// Light and Order Status Management
// -----------------------------------------------------------------------------
func startReceivingHallCallStatus(orderStatusChan chan OrderStatusMessage) {
    go func() {
        for {
            msg := <-rxOrderStatusChan
            orderStatusChan <- msg
        }
    }()
    go bcast.Receiver(statusPort, rxOrderStatusChan)
}

func SendOrderStatus(msg OrderStatusMessage) {

	timeout := time.After(giveUpTimeout)
	for{
		select{
		case ack := <- rxAckChan:
			if ack.TargetID == config.LocalID && ack.SeqNum == msg.SeqNum {
				fmt.Printf("Received ack from master")
				return
			}
		case <-timeout:
			fmt.Println("Timeout reached, stopping attempts")
			return
		default:
			fmt.Printf("Sending order status: unfinished to master")
			txOrderStatusChan <- msg
			time.Sleep(resendTimeout)
		}
	}
}

func SendLightOrder(buttonLight elevio.ButtonEvent, lightOnOrOff LightStatus) {
	stateMutex.Lock()
    defer stateMutex.Unlock()
    // Create tagged light order messages
	for _, elevator := range elevatorStatuses {
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

//PushTest!!