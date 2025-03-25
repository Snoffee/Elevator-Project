package network

import (
	"fmt"
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/network/bcast"
	"mainProject/network/peers"
	"sync"
	"time"
	"container/list"
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

type RecentAcks struct {
	m    map[int]*list.Element
	list *list.List
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
	
	resendTimeout			= 8 * time.Second
	giveUpTimeout			= 10 * time.Second

	seqNumAssignmentCounter = 20
	seqNumRawCallCounter	= 100
	SeqOrderStatusCounter   = 200
	seqLightCounter         = 300

	stateMutex	              sync.Mutex
	pendingAcks   		    = make(map[int]chan struct{})
    pendingAcksMutex 		  sync.Mutex
)

// -----------------------------------------------------------------------------
// Initialization and Network Management
// -----------------------------------------------------------------------------
func RunNetwork(elevatorStateChan chan map[string]ElevatorStatus, peerUpdates chan peers.PeerUpdate, orderStatusChan chan OrderStatusMessage, txAckChan chan AckMessage) {
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

	recentAcks := NewRecentAcks()
	for i := 0; i < 30; i++ {
		recentAcks.Add(i) //dummy values to add "padding" to begin with
	}

	go func() {
		for ack := range rxAckChan {
			pendingAcksMutex.Lock()
			if ackChan, exists := pendingAcks[ack.SeqNum]; exists && ack.TargetID == config.LocalID && !recentAcks.Exists(ack.SeqNum){
				fmt.Printf("ACK received for SeqNum: %d, TargetID %s\n", ack.SeqNum, ack.TargetID)
				close(ackChan)
				delete(pendingAcks, ack.SeqNum)
				recentAcks.Add(ack.SeqNum)
				recentAcks.RemoveOldest()
			} else {
				fmt.Printf("Unexpected ACK received: SeqNum: %d, TargetID %s (Possibly already processed)\n", ack.SeqNum, ack.TargetID)
			}
			pendingAcksMutex.Unlock()
		}
	}()
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
	ackChan := make(chan struct{})
	pendingAcksMutex.Lock()
	pendingAcks[hallCall.SeqNum] = ackChan
	pendingAcksMutex.Unlock()

	timeout := time.After(giveUpTimeout)
	go func() {
		defer func() {
			pendingAcksMutex.Lock()
			delete(pendingAcks, hallCall.SeqNum)
			pendingAcksMutex.Unlock()
		}()	
		for{
			select{
			case <- ackChan:
				fmt.Printf("Received ack for assignment from: %s | seqNum: %d\n", targetElevator, hallCall.SeqNum)
				return
			case <-timeout:
                fmt.Printf("Timeout reached for assignment to %s | SeqNum: %d\n", targetElevator, hallCall.SeqNum)
				return
			default:
				fmt.Printf("Sending assignment to %s for floor %d | SeqNum: %d\n", targetElevator, hallCall.Floor, hallCall.SeqNum)
				txAssignmentChan <- hallCall
				time.Sleep(resendTimeout)
			}
		}
	}()
}

// Sends a raw hall call event to the master elevator for assignment.
func SendRawHallCall(hallCall elevio.ButtonEvent) {
    if config.LocalID == config.MasterID {
        return
    }
    seqNumRawCallCounter++
    msg := RawHallCallMessage{TargetID: config.MasterID, SenderID: config.LocalID, Floor: hallCall.Floor, Button: hallCall.Button, SeqNum: seqNumRawCallCounter}
	
	ackChan := make(chan struct{})
    pendingAcksMutex.Lock()
    pendingAcks[msg.SeqNum] = ackChan
    pendingAcksMutex.Unlock()
	timeout := time.After(giveUpTimeout)
	
	go func() {
		defer func() {
			pendingAcksMutex.Lock()
			delete(pendingAcks, msg.SeqNum)
			pendingAcksMutex.Unlock()
		}()

		for{
			select{
			case <-ackChan:
                fmt.Printf("RawHallCall acknowledged by master master %s | SeqNum: %d\n", config.MasterID, msg.SeqNum)
				return
			case <-timeout:
                fmt.Printf("Timeout reached for RawHallCall to master %s | SeqNum: %d\n", config.MasterID, msg.SeqNum)
				return
			default:
                fmt.Printf("Sending RawHallCall to master %s for floor %d | SeqNum: %d\n", config.MasterID, hallCall.Floor, msg.SeqNum)
				txRawHallCallChan <- msg
				time.Sleep(resendTimeout)
			}
		}
	}()
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
	if config.LocalID == config.MasterID {
        return
    }
	SeqOrderStatusCounter++
	msg.SeqNum = SeqOrderStatusCounter

	ackChan := make(chan struct{})
	pendingAcksMutex.Lock()
	pendingAcks[msg.SeqNum] = ackChan
	pendingAcksMutex.Unlock()
	
	timeout := time.After(giveUpTimeout)
	go func() {
		defer func() {
			pendingAcksMutex.Lock()
			delete(pendingAcks, msg.SeqNum)
			pendingAcksMutex.Unlock()
		}()
		for{
			select{
			case <-ackChan:
				fmt.Printf("Order status acknowledged by master | SeqNum: %d\n", msg.SeqNum)
				return
			case <-timeout:
				fmt.Println("Timeout reached, stopping attempts")
				return
			default:
				fmt.Printf("Sending order status: unfinished to master | SeqNum: %d\n", msg.SeqNum)
				txOrderStatusChan <- msg
				time.Sleep(resendTimeout)
			}
		}
	}()
}

func SendLightOrder(buttonLight elevio.ButtonEvent, lightOnOrOff LightStatus) {
	stateMutex.Lock()
    defer stateMutex.Unlock()
    
	seqLightCounter++

	for _, elevator := range elevatorStatuses {
		if elevator.ID == config.LocalID {
			continue
		}
    	msg := LightOrderMessage{
        	TargetID: elevator.ID,
        	ButtonEvent: buttonLight,
			Light: lightOnOrOff,
			SeqNum: seqLightCounter,
    	}

		ackChan := make(chan struct{})
		pendingAcksMutex.Lock()
		pendingAcks[msg.SeqNum] = ackChan
		pendingAcksMutex.Unlock()
		
		timeout := time.After(giveUpTimeout)
		go func() {
			defer func() {
				pendingAcksMutex.Lock()
				delete(pendingAcks, msg.SeqNum)
				pendingAcksMutex.Unlock()
			}()
			for {
				select {
				case <-ackChan:
					fmt.Printf("Light order acknowledged by %s | SeqNum: %d\n", elevator.ID, msg.SeqNum)
					return
				case <-timeout:
					fmt.Println("Timeout reached, stopping attempts to set light")
					return
				default:
					fmt.Printf("Sending light order to %s for floor %d | SeqNum: %d\n", elevator.ID, msg.ButtonEvent.Floor, msg.SeqNum)
					txLightChan <- msg
					time.Sleep(resendTimeout)
				}
			}
		}()
	}
}

func NewRecentAcks() *RecentAcks {
	return &RecentAcks{
		m:    make(map[int]*list.Element),
		list: list.New(),
	}
}

func (recAcks *RecentAcks) Add(value int) {
	// If value already exists, you might choose to do nothing or update its position.
	if _, exists := recAcks.m[value]; exists {
		return
	}
	// Insert new value at the back of the list.
	elem := recAcks.list.PushBack(value)
	recAcks.m[value] = elem
	fmt.Printf("Added new element: %d\n", value)
}

func (recAcks *RecentAcks) Exists(value int) bool {
	_, exists := recAcks.m[value]
	return exists
}

func (recAcks *RecentAcks) RemoveOldest() {
	// Remove the front element (oldest).
	front := recAcks.list.Front()
	if front == nil {
		return
	}
	value := front.Value.(int)
	recAcks.list.Remove(front)
	delete(recAcks.m, value)
	fmt.Printf("Removed oldest element: %d\n", value)
}