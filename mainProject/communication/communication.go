package communication

import (
	//"fmt"
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/network/bcast"
	"mainProject/network/peers"
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
	State config.ElevatorState
	Direction elevio.MotorDirection
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
    SenderID    string
    ButtonEvent elevio.ButtonEvent
	Status      OrderStatus
	SeqNum      int
}

type LightStatus int

const (
	Off   LightStatus = 0
	On                = 1
)

type LightOrderMessage struct {
	TargetID    string
	ButtonEvent elevio.ButtonEvent
	Light       LightStatus
	SeqNum  	int
}

// -----------------------------------------------------------------------------
// Global Variables
// -----------------------------------------------------------------------------
var (
	elevatorStatuses        = make(map[string]ElevatorStatus) // Global map to track all known elevators
	backupElevatorStatuses  = make(map[string]ElevatorStatus)
	
	txElevatorStatusChan    = make(chan ElevatorStatus, 50) // Global transmitter channel
	rxElevatorStatusChan    = make(chan ElevatorStatus, 50) // Global receiver channel
	txAssignmentChan        = make(chan AssignmentMessage, 50) // Global channel for assignments
	txRawHallCallChan       = make(chan RawHallCallMessage, 50) // Slaves send hall call events to master
	txLightChan	            = make(chan LightOrderMessage, 50) // transmit light orders
	rxOrderStatusChan       = make(chan OrderStatusMessage, 50) // Receive confirmation of hall calls
	txOrderStatusChan       = make(chan OrderStatusMessage, 50) // Transmit confirmation of hall calls
	rxAckChan				= make(chan AckMessage, 500)
	
	seqNumAssignmentCounter = 0
	seqNumRawCallCounter	= 100
	SeqOrderStatusCounter   = 200
	seqLightCounter         = 300

	stateMutex	              sync.Mutex
	pendingAcks   		    = make(map[int]chan struct{})
    pendingAcksMutex 		  sync.Mutex

	MessageMaxRetries          = 5
    MessageRedundancyFactor    = 3
    MessageRetryInterval       = 200 * time.Millisecond
    MessageExponentialBackoff  = 2
)

// -----------------------------------------------------------------------------
// Initialization and Network Management
// -----------------------------------------------------------------------------
func RunCommunication(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate, orderStatusChan chan OrderStatusMessage, txAckChan chan AckMessage) {
	// Start peer reciver to get updates from other elevators
	go peers.Receiver(peerPort, peerUpdates)

	// Periodically send updated elevator status to other modules (locally)
	startPeriodicLocalStatusUpdates(elevatorStateChan)
	
	// Start broadcasting elevator status
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

	go func() {
		for ack := range rxAckChan {
			pendingAcksMutex.Lock()
			if ackChan, exists := pendingAcks[ack.SeqNum]; exists && ack.TargetID == config.LocalID{
				//fmt.Printf("ACK received for SeqNum: %d from %s\n", ack.SeqNum, ack.TargetID)
				close(ackChan)
				delete(pendingAcks, ack.SeqNum)
			} else {
				//fmt.Printf("Unexpected ACK received: SeqNum: %d from %s (Possibly already processed)\n", ack.SeqNum, ack.TargetID)
			}
			pendingAcksMutex.Unlock()
		}
	}()
}