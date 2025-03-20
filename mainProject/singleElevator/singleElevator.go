// In:
//		assignedHallCallChan (from `orderAssignment`) → Receives assigned hall calls.
//		drv_buttons (from `elevio`) → Detects button presses.
//		drv_floors (from `elevio`) → Detects floor arrival events.
//		drv_obstr (from `elevio`) → Detects obstruction events.

// Out:
//		hallCallChan → Sends hall call requests to `orderAssignment`.
//		network.BroadcastElevatorStatus() → Updates other elevators on status.

package singleElevator

import (
	"mainProject/elevio"
	"mainProject/network"
	"mainProject/network/bcast"
	"mainProject/config"
	"fmt"
	"time"
)

// Elevator heartbeat message
type Heartbeat struct {
	ID        string
	Timestamp time.Time
}

// **Run Single Elevator Logic**
func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {
	// Initialize elevator hardware event channels
	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)

	// Start polling hardware for events
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)

	fmt.Println("Single Elevator Module Running...")

	// Start the receiver to listen for hall assignments
	assignedNetworkHallCallChan  := make(chan network.AssignmentMessage, 10) 
	go bcast.Receiver(30002, assignedNetworkHallCallChan) // hallCallPort

	// Create a channel to receive raw hall calls.
	rawHallCallChan := make(chan network.RawHallCallMessage, 10)
	go bcast.Receiver(30003, rawHallCallChan) // rawHallCallPort

	// Start the receiver to listen for light orders
	lightOrderChan := make(chan network.LightOrderMessage, 10)
	go bcast.Receiver(30006, lightOrderChan) // lightPort

	rawHallCallChan2 := make(chan network.RawHallCallMessage, 10)
	go bcast.Transmitter(30004, rawHallCallChan2) // rawHallCallPort



	// Event Loop
	for {
		select {
		// Hardware		
		case buttonEvent := <-drv_buttons:
			ProcessButtonPress(buttonEvent, hallCallChan, orderStatusChan) // Handle button press event
		
		case floorEvent := <-drv_floors:
			ProcessFloorArrival(floorEvent, orderStatusChan) // Handle floor sensor event

		case obstructionEvent := <-drv_obstr:
			ProcessObstruction(obstructionEvent) // Handle obstruction event
		
		// Hall calls
		case assignedOrder := <-assignedHallCallChan:
			handleAssignedHallCall(assignedOrder, orderStatusChan) // Handle local assigned hall call
		
		case rawCall := <-rawHallCallChan:
			handleAssignedRawHallCall(rawCall, hallCallChan, rawHallCallChan2) // Handle global assigned hall call
		
		case networkAssignedOrder := <-assignedNetworkHallCallChan:
			handleAssignedNetworkHallCall(networkAssignedOrder, orderStatusChan) // Handle network assigned hall call

		case lightOrder := <-lightOrderChan:
			if lightOrder.TargetID == config.LocalID {
				if lightOrder.Light == network.Off {
					elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, false)
				} else {
					elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, true)
				}
			}
		}
		network.BroadcastElevatorStatus(GetElevatorState())
		time.Sleep(300 * time.Millisecond)
	}
}
