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


package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/network"
	"fmt"
	"time"
)

// **Handles button press events**
func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {
	fmt.Printf("Button pressed: %+v\n\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		floorSensorValue := elevio.GetFloor()
		// If the elevator is already at the requested floor, process it immediately
		if (elevator.Floor == event.Floor && floorSensorValue != -1){
			fmt.Println("Cab call at current floor, processing immediately...")
			ProcessFloorArrival(elevator.Floor, orderStatusChan) 
		} else {
			HandleStateTransition() 
		}
	} else {
		hallCallChan <- event
	}
}

// **Handles floor sensor events**
func ProcessFloorArrival(floor int, orderStatusChan chan network.OrderStatusMessage) {
	fmt.Printf("Floor sensor triggered: %+v\n", floor)
	elevio.SetFloorIndicator(floor)

	if !hasOrdersAtFloor(floor) {
		return
	}
	
	// Stop immediately if orders at current floor
	elevio.SetMotorDirection(elevio.MD_Stop)
	elevator.Floor = floor
	fmt.Printf("Elevator position updated: Now at Floor %d\n\n", elevator.Floor)

	hasUpCall := elevator.Queue[floor][elevio.BT_HallUp]
	hasDownCall := elevator.Queue[floor][elevio.BT_HallDown]
	hasCabCall := elevator.Queue[floor][elevio.BT_Cab]

	fmt.Println("Transitioning from Moving to DoorOpen...")
	elevator.State = config.DoorOpen
	elevio.SetDoorOpenLamp(true)

	// **Clear Cab Call if it exists**
	if hasCabCall {
		elevio.SetButtonLamp(elevio.BT_Cab, floor, false)
		elevator.Queue[floor][elevio.BT_Cab] = false
		fmt.Printf("Cleared cab call: Floor %d\n", floor)
	}

	// Decide which direction should be cleared first
	var firstClearButton elevio.ButtonType
	var secondClearButton elevio.ButtonType
	shouldDelaySecondClear := false

	if elevator.Direction == elevio.MD_Up && hasUpCall {
		firstClearButton = elevio.BT_HallUp
		secondClearButton = elevio.BT_HallDown
		shouldDelaySecondClear = hasDownCall
	} else if elevator.Direction == elevio.MD_Down && hasDownCall {
		firstClearButton = elevio.BT_HallDown
		secondClearButton = elevio.BT_HallUp
		shouldDelaySecondClear = hasUpCall
	} else {
		// If no ongoing direction or conflicting requests, clear based on priority
		if hasUpCall {
			firstClearButton = elevio.BT_HallUp
			secondClearButton = elevio.BT_HallDown
			shouldDelaySecondClear = hasDownCall
		} else if hasDownCall {
			firstClearButton = elevio.BT_HallDown
			secondClearButton = elevio.BT_HallUp
			shouldDelaySecondClear = hasUpCall
		}
	}

	// Clear the first button immediately (announce direction)
	elevio.SetButtonLamp(firstClearButton, floor, false)
	elevator.Queue[floor][firstClearButton] = false
	fmt.Printf("Cleared hall call: Floor %d, Button %v\n", floor, firstClearButton)

	msg := network.OrderStatusMessage{ButtonEvent: elevio.ButtonEvent{Floor: floor, Button: firstClearButton}, SenderID: config.LocalID, Status: network.Finished}
	orderStatusChan <- msg
	network.SendOrderStatus(msg)

	// If needed, delay clearing the second button (direction change)
	if shouldDelaySecondClear {
		fmt.Println("Keeping door open for an extra 3 seconds before changing direction...")
		go func() {
			time.Sleep(3 * time.Second)
			elevio.SetButtonLamp(secondClearButton, floor, false)
			elevator.Queue[floor][secondClearButton] = false
			fmt.Printf("Cleared opposite hall call after delay: Floor %d, Button %v\n", floor, secondClearButton)
			
			msg := network.OrderStatusMessage{ButtonEvent: elevio.ButtonEvent{Floor: floor, Button: secondClearButton}, SenderID: config.LocalID, Status: network.Finished}
			orderStatusChan <- msg
			network.SendOrderStatus(msg)
		}()
	}
	network.BroadcastElevatorStatus(elevator) 
	
	HandleStateTransition()
}

// **Handles obstruction events**
func ProcessObstruction(obstructed bool) {
	elevator.Obstructed = obstructed

	if elevator.State != config.DoorOpen {
		return
	}

	if obstructed{
		fmt.Printf("Obstruction detected: %+v\n", obstructed)
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)
		elevator.State = config.DoorOpen
	} else {
		fmt.Println("Obstruction cleared, transitioning to Idle...")
		go func() {
			time.Sleep(config.DoorOpenTime * time.Second)
			if !elevator.Obstructed {
				elevator.State = config.Idle
				elevio.SetDoorOpenLamp(false)
				HandleStateTransition()
			}
		}()
	}
}

// **Handles an assigned hall call from `order_assignment`**
// If the best elevator is itself, the order gets sent here
func handleAssignedHallCall(order elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage){
	fmt.Printf(" Received assigned hall call: Floor %d, Button %d\n\n", order.Floor, order.Button)

	elevator.Queue[order.Floor][order.Button] = true
	elevio.SetButtonLamp(order.Button, order.Floor, true)
	if order.Button != elevio.BT_Cab {
		msg := network.OrderStatusMessage{ButtonEvent: order, SenderID: config.LocalID, Status: network.Unfinished}
		orderStatusChan <- msg
		network.SendOrderStatus(msg)
		fmt.Printf("sent message with status:unfinished to network")
	}
	floorSensorValue := elevio.GetFloor()
	// If the elevator is already at the assigned floor, immediately process it
    if elevator.Floor == order.Floor && floorSensorValue != -1{
        fmt.Println("Already at assigned floor, processing immediately...")
        ProcessFloorArrival(elevator.Floor, orderStatusChan)
    } else {
        network.BroadcastElevatorStatus(GetElevatorState())
        HandleStateTransition()
    }
}

// **Handles an assigned raw hall call from the network**
func handleAssignedRawHallCall(rawCall network.RawHallCallMessage, hallCallChan chan elevio.ButtonEvent, rawChan chan network.RawHallCallMessage) {
    // Ignore calls not meant for this elevator
    if rawCall.TargetID != "" && rawCall.TargetID != config.LocalID {
        return
    }
    
    // Only process if it's not already in the queue
    if !elevator.Queue[rawCall.Floor][rawCall.Button] {
        fmt.Printf("Processing raw hall call: Floor %d, Button %d\n", rawCall.Floor, rawCall.Button)
        hallCallChan <- elevio.ButtonEvent{Floor: rawCall.Floor, Button: rawCall.Button}
		ackMsg:= network.RawHallCallMessage{TargetID: rawCall.SenderID, SenderID: config.LocalID, Floor: rawCall.Floor, Button: rawCall.Button, Ack: true}
		fmt.Print("Sending ack to slave \n")
		rawChan <- ackMsg
    }
}

// **Receive Hall Assignments from Network**
// If the best elevator was another elevator on the network the order gets sent here
func handleAssignedNetworkHallCall(msg network.AssignmentMessage, orderStatusChan chan network.OrderStatusMessage) {
	if msg.TargetID == config.LocalID {
		fmt.Printf("Received hall assignment for me from network: Floor %d, Button %v\n\n", msg.Floor, msg.Button)
		handleAssignedHallCall(elevio.ButtonEvent{Floor: msg.Floor, Button: msg.Button}, orderStatusChan)
	} else {
		// If this elevator previously had the request, remove it
		if elevator.Queue[msg.Floor][msg.Button] {
			fmt.Printf("Removing hall call at Floor %d from local queue, since assigned elsewhere\n", msg.Floor)
			clearFloorOrders(msg.Floor)
		}
	}
}



