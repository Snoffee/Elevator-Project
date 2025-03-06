package single_elevator

// In: 	Orders
//		Elevator.Config

// Out: Hall orders

import (
	"Main_project/elevio"  
	"Main_project/network"
	"time"
	"fmt"
)

// **Run Single Elevator Logic**
func RunSingleElevator() {
	// Initialize elevator hardware event channels
	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)

	// Start polling hardware for events
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)

	fmt.Println("Single Elevator Module Running...")

	// Event Loop
	for {
		select {
		case buttonEvent := <-drv_buttons:
			ProcessButtonPress(buttonEvent) // Handle button press event

		case floorEvent := <-drv_floors:
			ProcessFloorArrival(floorEvent) // Handle floor sensor event

		case obstructionEvent := <-drv_obstr:
			ProcessObstruction(obstructionEvent) // Handle obstruction event
		}
		network.BroadcastElevatorStatus(GetElevatorState())
		time.Sleep(500 * time.Millisecond)
	}
}