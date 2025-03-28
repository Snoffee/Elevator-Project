package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/communication"
	"fmt"
	"time"
)

func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent, orderStatusChan chan communication.OrderStatusMessage, localStatusUpdateChan chan config.Elevator) {
	fmt.Printf("Button pressed: %+v\n\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		localStatusUpdateChan <- GetElevatorState()
		
		// If the elevator is already at the requested floor, process it immediately
		floorSensorValue := elevio.GetFloor()
		if (elevator.Floor == event.Floor && floorSensorValue != -1 && elevator.State != config.Moving){
			fmt.Println("Cab call at current floor, processing immediately...")
			time.Sleep(3 * time.Second)
			ProcessFloorArrival(elevator.Floor, orderStatusChan, localStatusUpdateChan)
			
			localStatusUpdateChan <- GetElevatorState()
		}else{
			HandleStateTransition(orderStatusChan) 
		}
	} else {
		hallCallChan <- event
	}
}

func ProcessFloorArrival(floor int, orderStatusChan chan communication.OrderStatusMessage, localStatusUpdateChan chan config.Elevator) {
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

	fmt.Println("Transitioning from Moving to DoorOpen...")
	elevator.State = config.DoorOpen
	elevio.SetDoorOpenLamp(true)
	doorTimer.Reset(config.DoorOpenTime * time.Second)
}

func ProcessObstruction(obstructed bool, orderStatusChan chan communication.OrderStatusMessage) {
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
		HandleStateTransition(orderStatusChan)
	} else {
		fmt.Println("Obstruction cleared, transitioning to Idle...")
		obstructionTimer.Stop()
		doorTimer.Reset(config.DoorOpenTime * time.Second)
	}
}

//Used after HandleFloorArrival to decide which orders are cleared
func hallCallClearOrder(floor int)(elevio.ButtonType, elevio.ButtonType, bool){
	hasDownCall := elevator.Queue[floor][elevio.BT_HallDown]
	hasUpCall   := elevator.Queue[floor][elevio.BT_HallUp]
	ordersAbove := HasOrdersAbove(elevator)
	ordersBelow := HasOrdersBelow(elevator)
	hasCabCall  := elevator.Queue[floor][elevio.BT_Cab]

	// Clear Cab Call if it exists
	if hasCabCall {
		elevio.SetButtonLamp(elevio.BT_Cab, floor, false)
		elevator.Queue[floor][elevio.BT_Cab] = false
		fmt.Printf("Cleared cab call: Floor %d\n", floor)
		if !hasUpCall && !hasDownCall{
			return elevio.BT_Cab, elevio.BT_Cab, false
		}
	}
	hasOnlyOneDirectionInQueue := false
	if (ordersAbove && !ordersBelow) || (!ordersAbove && ordersBelow){
		hasOnlyOneDirectionInQueue = true
	}

	var firstClearButton elevio.ButtonType
	var secondClearButton elevio.ButtonType
	shouldDelaySecondClear := false
	secondClearButton = elevio.BT_Cab

	//There are a lot of edge cases that must be considered, and a large if/else block is therefore required.

	//Checks for the project description case of only one of the two ordered directions having other orders.

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
	//There is no specific requirement in the description. Our system therefore only services one direction if no other orders are present.
	}else if hasUpCall && hasDownCall{
		if elevator.Direction == elevio.MD_Up{ //If moving in a certain direction, it is prioritized.
			firstClearButton = elevio.BT_HallUp
		}else if elevator.Direction == elevio.MD_Down{ //If moving in a certain direction, it is prioritized.
			firstClearButton = elevio.BT_HallDown
		}else{
			firstClearButton = elevio.BT_HallDown //Default to prioritize down
		}
		//More typical scenario with hall order in only one direction.
	}else{
		if hasUpCall && ordersAbove{
			firstClearButton = elevio.BT_HallUp
		}else if hasDownCall && ordersBelow{
			firstClearButton = elevio.BT_HallDown
		}else if !ordersAbove && !ordersBelow{
			if hasUpCall{
				firstClearButton = elevio.BT_HallUp
			}else if hasDownCall{
				firstClearButton = elevio.BT_HallDown
			}
		}else{
			firstClearButton = elevio.BT_Cab //Sets to cab to signalize no hall orders need to be cleared
		}
		
	}
	return firstClearButton, secondClearButton, shouldDelaySecondClear
}

//Performs the clearing of hall calls as decided by func hallCallClearOrder
func clearAllOrdersAtFloor(floor int, orderStatusChan chan communication.OrderStatusMessage, localStatusUpdateChan chan config.Elevator, firstClearButton elevio.ButtonType){

	if(firstClearButton == elevio.BT_Cab){
		return
	}
	// Clear the first button immediately (announce direction)
	elevio.SetButtonLamp(firstClearButton, floor, false)
	elevator.Queue[floor][firstClearButton] = false
	fmt.Printf("Cleared hall call: Floor %d, Button %v\n", floor, firstClearButton)

	//Send finished order status message to sync hall button lights
	msg := communication.OrderStatusMessage{ButtonEvent: elevio.ButtonEvent{Floor: floor, Button: firstClearButton}, SenderID: config.LocalID, Status: communication.Finished}
	communication.SendOrderStatus(msg, orderStatusChan)
	MarkAssignmentAsCompleted(msg.SeqNum)

	localStatusUpdateChan <- GetElevatorState()

}