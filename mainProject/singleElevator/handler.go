package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/network"
	"fmt"
	"time"
)

// Handles button press events
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
			go func() {
				time.Sleep(2 * time.Second)  
				ProcessFloorArrival(elevator.Floor, orderStatusChan)
			}()
			network.BroadcastElevatorStatus(GetElevatorState())
		} else {
			HandleStateTransition() 
		}
	} else {
		hallCallChan <- event
	}
}

// Handles floor sensor events
func ProcessFloorArrival(floor int, orderStatusChan chan network.OrderStatusMessage) {
	fmt.Printf("Floor sensor triggered: %+v\n", floor)
	elevio.SetFloorIndicator(floor)
	movementTimer.Reset(config.NotMovingTimeLimit * time.Second)

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

	// Decide which direction should be cleared first
	var firstClearButton elevio.ButtonType
	var secondClearButton elevio.ButtonType
	shouldDelaySecondClear := false

	if hasUpCall && hasDownCall{
		shouldDelaySecondClear = true
		if elevator.Direction == elevio.MD_Up && ordersAbove{
			firstClearButton = elevio.BT_HallDown
			secondClearButton = elevio.BT_HallUp
		}else if elevator.Direction == elevio.MD_Down && ordersBelow{
			firstClearButton = elevio.BT_HallUp
			secondClearButton = elevio.BT_HallDown
		}else{
			if ordersAbove {
				firstClearButton = elevio.BT_HallDown
				secondClearButton = elevio.BT_HallUp
			} else if ordersBelow {
				firstClearButton = elevio.BT_HallDown
				secondClearButton = elevio.BT_HallUp
			}
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

	msg := network.OrderStatusMessage{ButtonEvent: elevio.ButtonEvent{Floor: floor, Button: firstClearButton}, SenderID: config.LocalID, Status: network.Finished}
	orderStatusChan <- msg
	network.SendOrderStatus(msg)

	// If needed, delay clearing the second button (direction change)
	if shouldDelaySecondClear {
		fmt.Println("Keeping door open for an extra 3 seconds before changing direction...")
		doorTimer.Reset(config.DoorOpenTime * 2 * time.Second)
		movementTimer.Stop()
		delayedButtonEvent = elevio.ButtonEvent{Floor: floor, Button: secondClearButton}
		clearOppositeDirectionTimer.Reset(config.DoorOpenTime * time.Second)
	} 

	network.BroadcastElevatorStatus(elevator)
}

// Handles obstruction events
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

// Handles an assigned hall call from `order_assignment`
func handleAssignedHallCall(order elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage){
	fmt.Printf(" Received assigned hall call: Floor %d, Button %d\n\n", order.Floor, order.Button)

	elevator.Queue[order.Floor][order.Button] = true
	elevio.SetButtonLamp(order.Button, order.Floor, true)

	if order.Button != elevio.BT_Cab {
		network.SeqOrderStatusCounter++
		msg := network.OrderStatusMessage{ButtonEvent: order, SenderID: config.LocalID, Status: network.Unfinished}
		orderStatusChan <- msg
		go network.SendOrderStatus(msg)
	}
	floorSensorValue := elevio.GetFloor()
	// If the elevator is already at the assigned floor, immediately process it
    if elevator.Floor == order.Floor && floorSensorValue != -1{
        fmt.Println("Already at assigned floor, processing immediately...")
		go func() {
			time.Sleep(2 * time.Second)  
			ProcessFloorArrival(elevator.Floor, orderStatusChan)
		}() 
		} else {
        network.BroadcastElevatorStatus(GetElevatorState())
        HandleStateTransition()
    }
}

// Handles an assigned raw hall call from the network
func handleAssignedRawHallCall(rawCall network.RawHallCallMessage, hallCallChan chan elevio.ButtonEvent, txAckChan chan network.AckMessage) {
    // Ignore calls not meant for this elevator
    if rawCall.TargetID != "" && rawCall.TargetID != config.LocalID {
        return
    }
	fmt.Printf("Processing raw hall call: Floor %d, Button %d\n", rawCall.Floor, rawCall.Button)
	hallCallChan <- elevio.ButtonEvent{Floor: rawCall.Floor, Button: rawCall.Button}
	ackMsg:= network.AckMessage{TargetID: rawCall.SenderID, SeqNum: rawCall.SeqNum}
	fmt.Printf("Sending ack for RawHallCall to sender: %s | SeqNum: %d\n\n", ackMsg.TargetID, ackMsg.SeqNum)
	txAckChan <- ackMsg
	
}

// Receive Hall Assignments from Network
// If the best elevator was another elevator on the network the order gets sent here (master elevator)
func handleAssignedNetworkHallCall(msg network.AssignmentMessage, orderStatusChan chan network.OrderStatusMessage, txAckChan chan network.AckMessage) {
	if msg.TargetID == config.LocalID {
		fmt.Printf("Received hall assignment for me from network: Floor %d, Button %v\n\n", msg.Floor, msg.Button)
		handleAssignedHallCall(elevio.ButtonEvent{Floor: msg.Floor, Button: msg.Button}, orderStatusChan)

		ackMsg := network.AckMessage{TargetID: config.MasterID, SeqNum: msg.SeqNum}
		txAckChan <- ackMsg
		fmt.Printf("Broadcasting ack for assignment | SeqNum: %d\n\n", ackMsg.SeqNum)
		
	}
}