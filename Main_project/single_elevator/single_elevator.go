// In:
//		assignedHallCallChan (from `order_assignment`) → Receives assigned hall calls.
//		drv_buttons (from `elevio`) → Detects button presses.
//		drv_floors (from `elevio`) → Detects floor arrival events.
//		drv_obstr (from `elevio`) → Detects obstruction events.

// Out:
//		hallCallChan → Sends hall call requests to `order_assignment`.
//		network.BroadcastElevatorStatus() → Updates other elevators on status.

package single_elevator

import (
	"Main_project/elevio"
	"Main_project/network"
	"Main_project/network/bcast"
	"fmt"
	"time"
)

// **Run Single Elevator Logic**
func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent) {
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
	assignedNetworkHallCallChan  := make(chan network.HallAssignmentMessage, 10) 
	go bcast.Receiver(30002, assignedNetworkHallCallChan)

	// Create a channel to receive raw hall calls.
	rawHallCallChan := make(chan elevio.ButtonEvent, 10)
	go bcast.Receiver(30003, rawHallCallChan)

	// Merge Raw hall calls into main hall call channel
	go func() {
		for {
			event := <-rawHallCallChan
			if !elevator.Queue[event.Floor][event.Button] { // Check if already registered
				hallCallChan <- event
			}
		}
	}()

	go ReceiveHallAssignments(assignedNetworkHallCallChan) // Listen for network hall calls

	// Event Loop
	for {
		select {
		case buttonEvent := <-drv_buttons:
			ProcessButtonPress(buttonEvent, hallCallChan) // Handle button press event
		
		case assignedOrder := <-assignedHallCallChan:
			handleAssignedHallCall(assignedOrder) // Handle local assigned hall call

		case floorEvent := <-drv_floors:
			ProcessFloorArrival(floorEvent) // Handle floor sensor event

		case obstructionEvent := <-drv_obstr:
			ProcessObstruction(obstructionEvent) // Handle obstruction event
		}
		network.BroadcastElevatorStatus(GetElevatorState())
		time.Sleep(500 * time.Millisecond)
	}
}