package network

import (
	"fmt"
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
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate, orderStatusChan chan OrderStatusMessage, txAckChan chan AckMessage) {
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

// -----------------------------------------------------------------------------
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
        Direction: e.State,
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
		SeqNum:   seqNumAssignmentCounter,
	}
	go reliablePacketTransmit(hallCall, txAssignmentChan, hallCall.SeqNum, targetElevator, "Assignment Message", MessageRedundancyFactor)
}
// Sends a raw hall call event to the master elevator for assignment.
func SendRawHallCall(hallCall elevio.ButtonEvent) {
    if config.LocalID == config.MasterID {
        return
    }
    seqNumRawCallCounter++
    msg := RawHallCallMessage{
		TargetID: config.MasterID, 
		SenderID: config.LocalID, 
		Floor: 	  hallCall.Floor, 
		Button:	  hallCall.Button, 
		SeqNum:	  seqNumRawCallCounter,
	}
	go reliablePacketTransmit(msg, txRawHallCallChan, msg.SeqNum, config.MasterID, "Raw Hall Call", MessageRedundancyFactor)
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
	SeqOrderStatusCounter++
	msg.SeqNum = SeqOrderStatusCounter

	var redundancyFactor int
	if msg.Status == Finished {
		redundancyFactor = MessageRedundancyFactor * 3  // Send Finished messages with higher redundancy
	} else {
		redundancyFactor = MessageRedundancyFactor
	}
	go reliablePacketTransmit(msg, txOrderStatusChan, msg.SeqNum, config.MasterID, "Order Status Message", redundancyFactor)
}

func SendLightOrder(buttonLight elevio.ButtonEvent, lightOnOrOff LightStatus, statusSenderID string) {
	for _, elevator := range elevatorStatuses {
		if elevator.ID == config.LocalID || elevator.ID == statusSenderID {
			continue
		}
		seqLightCounter++
		msg := LightOrderMessage{
			TargetID:    elevator.ID,
			ButtonEvent: buttonLight,
			Light:       lightOnOrOff,
			SeqNum:      seqLightCounter,
		}
		go reliablePacketTransmit(msg, txLightChan, msg.SeqNum, msg.TargetID, "Light Order", MessageRedundancyFactor)
	}
}

// -----------------------------------------------------------------------------
// Combined Message Handling
// -----------------------------------------------------------------------------
func reliablePacketTransmit(msg interface{}, txChan interface{}, seqNum int, targetID string, description string, redundancyFactor int) {
    ackChan := make(chan struct{})
    pendingAcksMutex.Lock()
    pendingAcks[seqNum] = ackChan
    pendingAcksMutex.Unlock()

    retries := 0
    currentInterval := MessageRetryInterval

    for retries < MessageMaxRetries {
        for i := 0; i < redundancyFactor; i++ {
            switch ch := txChan.(type) { 
            case chan AssignmentMessage:
                ch <- msg.(AssignmentMessage)
            case chan RawHallCallMessage:
                ch <- msg.(RawHallCallMessage)
            case chan OrderStatusMessage:
                ch <- msg.(OrderStatusMessage)
            case chan LightOrderMessage:
                ch <- msg.(LightOrderMessage)
            }
        }

        select {
        case <-ackChan:
			if targetID == config.LocalID {
            	fmt.Printf("[ACK Received] %s | SeqNum: %d | Target: %s\n", description, seqNum, targetID)
			}	
            return
        case <-time.After(currentInterval):
            retries++
            currentInterval *= time.Duration(MessageExponentialBackoff)
            fmt.Printf("[Retrying] %s | SeqNum: %d | Attempt: %d/%d\n", description, seqNum, retries, MessageMaxRetries)
        }
    }
    fmt.Printf("[Failed] %s | SeqNum: %d | Could not be delivered after %d attempts.\n", description, seqNum, MessageMaxRetries)
    pendingAcksMutex.Lock()
    delete(pendingAcks, seqNum)
    pendingAcksMutex.Unlock()
}

