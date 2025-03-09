// In:
//		Button press events (from `drv_buttons` in `single_elevator.go`).
//		Floor arrival events (from `drv_floors` in `single_elevator.go`).
//		Obstruction status (from `drv_obstr` in `single_elevator.go`).
//		Assigned hall calls (from `order_assignment` via `assignedHallCallChan`).
//		Hall call assignments (from `network.BroadcastHallAssignment`).

// Out:
//		hallCallChan → Sends hall call requests to `order_assignment`.
//		Updates `elevator.Queue` when a button is pressed or assigned.
//		Handles door opening/closing and state transitions.


package single_elevator

import (
	"Main_project/config"
	"Main_project/elevio"
	"Main_project/network/bcast"
	"fmt"
	"time"
)

// **Handles button press events**
func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent) {
	fmt.Printf("Button pressed: %+v\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		HandleStateTransition()
	} else {
		// Hall calls are sent to 'order_assignment'
		hallCallChan <- event
	}
}

// **Handles floor sensor events**
func ProcessFloorArrival(floor int) {
	fmt.Printf("Floor sensor triggered: %+v\n", floor)
	elevator.Floor = floor
	elevio.SetFloorIndicator(floor)

	// If an order exists at this floor, open doors
	if elevator.Queue[floor] != [config.NumButtons]bool{false} {
		fmt.Println("Transitioning from Moving to DoorOpen...")
		elevator.State = config.DoorOpen
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)

		// **Turn off button lights after servicing**
		elevator.Queue[floor] = [config.NumButtons]bool{false}
		for btn := 0; btn < config.NumButtons; btn++ {
			elevio.SetButtonLamp(elevio.ButtonType(btn), floor, false)
		}
	}
	HandleStateTransition()
}

// **Handles obstruction events**
func ProcessObstruction(obstructed bool) {
	fmt.Printf("Obstruction detected: %+v\n", obstructed)
	elevator.Obstructed = obstructed

	if obstructed {
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)
		elevator.State = config.DoorOpen
	} else {
		fmt.Println("Obstruction cleared, transitioning to Idle...")
		go func() {
			time.Sleep(config.DoorOpenTime * time.Second)
			if !elevator.Obstructed {
				elevator.State = config.Idle
				HandleStateTransition()
			}
		}()
	}
}

// **Handles an assigned hall call from `order_assignment`**
func handleAssignedHallCall(order elevio.ButtonEvent) {
	fmt.Printf(" Received assigned hall call: Floor %d, Button %d\n", order.Floor, order.Button)
	elevator.Queue[order.Floor][order.Button] = true
	elevio.SetButtonLamp(order.Button, order.Floor, true)
	HandleStateTransition()
}

// **Receive Hall Assignments from Network**
func ReceiveHallAssignments(rxChan chan elevio.ButtonEvent) {
	go bcast.Receiver(30002, rxChan) // Use the same port as `BroadcastHallAssignment`

	for {
		hallCall := <-rxChan
		fmt.Printf("Received hall assignment: Floor %d, Button %v\n", hallCall.Floor, hallCall.Button)

		// Add the order to the queue and turn on the button lamp
		elevator.Queue[hallCall.Floor][hallCall.Button] = true
		elevio.SetButtonLamp(hallCall.Button, hallCall.Floor, true)
		HandleStateTransition()
	}
}


