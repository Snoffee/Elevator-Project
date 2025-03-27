package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/communication"
	"fmt"
	"time"
)

// Handles button press events
func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent, orderStatusChan chan communication.OrderStatusMessage) {
	fmt.Printf("Button pressed: %+v\n\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		communication.BroadcastElevatorStatus(GetElevatorState(), true)
		
		floorSensorValue := elevio.GetFloor()
		// If the elevator is already at the requested floor, process it immediately
		if (elevator.Floor == event.Floor && floorSensorValue != -1){
			fmt.Println("Cab call at current floor, processing immediately...")
			time.Sleep(3 * time.Second)
			ProcessFloorArrival(elevator.Floor, orderStatusChan)
			
			communication.BroadcastElevatorStatus(GetElevatorState(), true)
		} else {
			HandleStateTransition() 
		}
	} else {
		hallCallChan <- event
	}
}

// Handles floor sensor events
func ProcessFloorArrival(floor int, orderStatusChan chan communication.OrderStatusMessage) {
	fmt.Printf("Floor sensor triggered: %+v\n", floor)
	elevio.SetFloorIndicator(floor)
	movementTimer.Reset(notMovingTimeLimit * time.Second)

	if !hasOrdersAtFloor(floor) {
		return
	}
	// Stop immediately if orders at current floor
	elevio.SetMotorDirection(elevio.MD_Stop)
	elevator.Floor = floor
	fmt.Printf("Elevator position updated: Now at Floor %d\n\n", elevator.Floor)

	

	hasUpCall := elevator.Queue[floor][elevio.BT_HallUp]
	ordersAbove := HasOrdersAbove(elevator)
	hasDownCall := elevator.Queue[floor][elevio.BT_HallDown]
	ordersBelow := HasOrdersBelow(elevator)
	hasCabCall := elevator.Queue[floor][elevio.BT_Cab]
	hasOnlyOneDirectionInQueue := false
	if (ordersAbove && !ordersBelow) || (!ordersAbove && ordersBelow){
		hasOnlyOneDirectionInQueue = true
	}


	fmt.Println("Transitioning from Moving to DoorOpen...")
	elevator.State = config.DoorOpen
	elevio.SetDoorOpenLamp(true)
	doorTimer.Reset(config.DoorOpenTime * time.Second)

	// Clear Cab Call if it exists
	if hasCabCall {
		elevio.SetButtonLamp(elevio.BT_Cab, floor, false)
		elevator.Queue[floor][elevio.BT_Cab] = false
		fmt.Printf("Cleared cab call: Floor %d\n", floor)
		if !hasUpCall && !hasDownCall{
			return
		}
	}

	// Decide which direction should be cleared first.
	
	var firstClearButton elevio.ButtonType
	var secondClearButton elevio.ButtonType
	shouldDelaySecondClear := false
	//Checks for both directions. If true, clears the least prioritized direction first.
	if hasUpCall && hasDownCall && hasOnlyOneDirectionInQueue{
		shouldDelaySecondClear = true
		if elevator.Direction == elevio.MD_Up && ordersAbove{ //If moving in a certain direction, it is prioritized.
			firstClearButton = elevio.BT_HallDown
			secondClearButton = elevio.BT_HallUp
		}else if elevator.Direction == elevio.MD_Down && ordersBelow{
			firstClearButton = elevio.BT_HallUp
			secondClearButton = elevio.BT_HallDown
		}else{
			if ordersAbove{
				firstClearButton = elevio.BT_HallDown
				secondClearButton = elevio.BT_HallUp
			} else if ordersBelow{
				firstClearButton = elevio.BT_HallDown
				secondClearButton = elevio.BT_HallUp
			}
		}
	}else if hasUpCall && hasDownCall{
		if elevator.Direction == elevio.MD_Up{ //If moving in a certain direction, it is prioritized.
			firstClearButton = elevio.BT_HallUp
		}else if elevator.Direction == elevio.MD_Down{ //If moving in a certain direction, it is prioritized.
			firstClearButton = elevio.BT_HallDown
		}else{
			firstClearButton = elevio.BT_HallDown
		}
	}else{
		if hasUpCall{
			firstClearButton = elevio.BT_HallUp
		}else if hasDownCall{
			firstClearButton = elevio.BT_HallDown
		}
		
	}

	// Clear the first button immediately (announce direction)
	elevio.SetButtonLamp(firstClearButton, floor, false)
	elevator.Queue[floor][firstClearButton] = false
	fmt.Printf("Cleared hall call: Floor %d, Button %v\n", floor, firstClearButton)

	msg := communication.OrderStatusMessage{ButtonEvent: elevio.ButtonEvent{Floor: floor, Button: firstClearButton}, SenderID: config.LocalID, Status: communication.Finished}
	orderStatusChan <- msg
	communication.SendOrderStatus(msg)
	MarkAssignmentAsCompleted(msg.SeqNum)

	// If needed, delay clearing the second button (direction change)
	if shouldDelaySecondClear {
		fmt.Println("Keeping door open for an extra 3 seconds before changing direction...")
		doorTimer.Reset(config.DoorOpenTime * 2 * time.Second)
		movementTimer.Stop()
		delayedButtonEvent = elevio.ButtonEvent{Floor: floor, Button: secondClearButton}
		clearOppositeDirectionTimer.Reset(config.DoorOpenTime * time.Second)
	} 

	communication.BroadcastElevatorStatus(elevator, true)
}

func ProcessObstruction(obstructed bool) {
	elevator.Obstructed = obstructed

	if elevator.State != config.DoorOpen {
		return
	}

	if obstructed{
		movementTimer.Stop()
		fmt.Printf("Obstruction detected: %+v\n", obstructed)
		elevio.SetMotorDirection(elevio.MD_Stop)
		elevio.SetDoorOpenLamp(true)
		elevator.State = config.DoorOpen
		HandleStateTransition()
	} else {
		fmt.Println("Obstruction cleared, transitioning to Idle...")
		doorTimer.Reset(config.DoorOpenTime * time.Second)
	}
}