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
	"Main_project/network"
	"fmt"
	"time"
)

// **Handles button press events**
func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent) {
	fmt.Printf("Button pressed: %+v\n\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		
		// If the elevator is already at the requested floor, process it immediately
		if elevator.Floor == event.Floor {
			fmt.Println("Cab call at current floor, processing immediately...")
			ProcessFloorArrival(elevator.Floor) // Reuse existing floor arrival logic
		} else {
			HandleStateTransition() // Otherwise, continue normally
		}
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
	fmt.Printf("Elevator position updated: Now at Floor %d\n\n", elevator.Floor)

	// If an order exists at this floor, open doors
	if elevator.Queue[floor] != [config.NumButtons]bool{false} {
		fmt.Println("Transitioning from Moving to DoorOpen...")
		elevator.State = config.DoorOpen
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)

		// Turn off button lights after servicing
		elevator.Queue[floor] = [config.NumButtons]bool{false}
		for btn := 0; btn < config.NumButtons; btn++ {
			elevio.SetButtonLamp(elevio.ButtonType(btn), floor, false)
		}
		network.BroadcastElevatorStatus(elevator) // Ensure master sees the update
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
// If the best elevator is itself, the order gets sent here
func handleAssignedHallCall(order elevio.ButtonEvent) {
	fmt.Printf(" Received assigned hall call: Floor %d, Button %d\n\n", order.Floor, order.Button)
	elevator.Queue[order.Floor][order.Button] = true
	elevio.SetButtonLamp(order.Button, order.Floor, true)

	// If the elevator is already at the assigned floor, immediately process it
    if elevator.Floor == order.Floor {
        fmt.Println("Already at assigned floor, processing immediately...")
        ProcessFloorArrival(elevator.Floor)
    } else {
        // Proceed normally if the elevator is not already at the floor
        network.BroadcastElevatorStatus(GetElevatorState())
        HandleStateTransition()
    }
}

// **Handles an assigned raw hall call from the network**
func handleAssignedRawHallCall(rawCall network.RawHallCallMessage, hallCallChan chan elevio.ButtonEvent) {
    // Ignore calls not meant for this elevator
    if rawCall.TargetID != "" && rawCall.TargetID != config.LocalID {
        return
    }
    
    // Only process if it's not already in the queue
    if !elevator.Queue[rawCall.Floor][rawCall.Button] {
        fmt.Printf("Processing raw hall call: Floor %d, Button %d\n", rawCall.Floor, rawCall.Button)
        hallCallChan <- elevio.ButtonEvent{Floor: rawCall.Floor, Button: rawCall.Button}
    }
}

// **Receive Hall Assignments from Network**
// If the best elevator was another elevator on the network the order gets sent here
func ReceiveHallAssignments(assignedNetworkHallCallChan chan network.HallAssignmentMessage) {
	for {
		msg := <-assignedNetworkHallCallChan
        // Only process the message if it is intended for this elevator.
        if msg.TargetID == config.LocalID {
            fmt.Printf("Received hall assignment for me from network: Floor %d, Button %v\n\n", msg.Floor, msg.Button)
            // Convert to a ButtonEvent and handle it.
            event := elevio.ButtonEvent{Floor: msg.Floor, Button: msg.Button}
            handleAssignedHallCall(event)
        } else {
            // If this elevator previously had the request, remove it
            if elevator.Queue[msg.Floor][msg.Button] {
                fmt.Printf("Removing hall call at Floor %d from local queue, since assigned elsewhere\n", msg.Floor)
                elevator.Queue[msg.Floor][msg.Button] = false
                elevio.SetButtonLamp(msg.Button, msg.Floor, false)
            }
        }
	}
}


