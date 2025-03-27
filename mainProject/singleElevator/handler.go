package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/network"
	"fmt"
	"time"
	"sync"
)

// Maps To Track Recent Messages
var (
	recentAssignments         = make(map[int]time.Time)
	recentRawHallCalls 		  = make(map[int]time.Time)
	recentOrderStatusMessages = make(map[int]time.Time)
	recentLightOrderMessages  = make(map[int]time.Time)
	recentMessagesMutex 	  = &sync.Mutex{}
)

// Handles button press events
func ProcessButtonPress(event elevio.ButtonEvent, hallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {
	fmt.Printf("Button pressed: %+v\n\n", event)
	
	// Cab calls are handled locally
	if event.Button == elevio.BT_Cab{
		elevator.Queue[event.Floor][event.Button] = true
		elevio.SetButtonLamp(event.Button, event.Floor, true)
		network.BroadcastElevatorStatus(GetElevatorState(), true)
		
		floorSensorValue := elevio.GetFloor()
		// If the elevator is already at the requested floor, process it immediately
		if (elevator.Floor == event.Floor && floorSensorValue != -1){
			fmt.Println("Cab call at current floor, processing immediately...")
			time.Sleep(3 * time.Second)
			ProcessFloorArrival(elevator.Floor, orderStatusChan)
			
			network.BroadcastElevatorStatus(GetElevatorState(), true)
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
	MarkAssignmentAsCompleted(msg.SeqNum)

	// If needed, delay clearing the second button (direction change)
	if shouldDelaySecondClear {
		fmt.Println("Keeping door open for an extra 3 seconds before changing direction...")
		doorTimer.Reset(config.DoorOpenTime * 2 * time.Second)
		movementTimer.Stop()
		delayedButtonEvent = elevio.ButtonEvent{Floor: floor, Button: secondClearButton}
		clearOppositeDirectionTimer.Reset(config.DoorOpenTime * time.Second)
	} 

	network.BroadcastElevatorStatus(elevator, true)
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

// -----------------------------------------------------------------------------
// Handles an assigned hall call from `order_assignment`
// -----------------------------------------------------------------------------
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
		time.Sleep(3 * time.Second)
		ProcessFloorArrival(elevator.Floor, orderStatusChan)
		network.BroadcastElevatorStatus(GetElevatorState(), true)
	} else {
        HandleStateTransition()
    }
}

// -----------------------------------------------------------------------------
// Receiving Raw Hall Call from a slave (Only for master)
// -----------------------------------------------------------------------------
func handleAssignedRawHallCall(rawCall network.RawHallCallMessage, hallCallChan chan elevio.ButtonEvent, txAckChan chan network.AckMessage) {
    if config.LocalID != config.MasterID {
        return
    }
	recentMessagesMutex.Lock()
	if _, exists := recentRawHallCalls[rawCall.SeqNum]; exists {
        fmt.Printf("[Duplicate Detected] Ignoring duplicate Raw Hall Call: Floor %d, Button %v | SeqNum: %d\n", rawCall.Floor, rawCall.Button, rawCall.SeqNum)
        recentMessagesMutex.Unlock()
		return
    }
    recentRawHallCalls[rawCall.SeqNum] = time.Now()
	recentMessagesMutex.Unlock()
	fmt.Printf("Processing raw hall call: Floor %d, Button %d\n", rawCall.Floor, rawCall.Button)
	hallCallChan <- elevio.ButtonEvent{Floor: rawCall.Floor, Button: rawCall.Button}
	
	ackMsg:= network.AckMessage{TargetID: rawCall.SenderID, SeqNum: rawCall.SeqNum}
	fmt.Printf("Broadcasting ack for RawHallCall to sender: %s | SeqNum: %d\n\n", ackMsg.TargetID, ackMsg.SeqNum)
	for i := 0; i < 3; i++ {
		txAckChan <- ackMsg
		time.Sleep(20 * time.Millisecond)
	}
}

// -----------------------------------------------------------------------------
// Receiving Hall Assignments from master (from network)
// -----------------------------------------------------------------------------
// If the best elevator was another elevator on the network the order gets sent here (master elevator)
func handleAssignedNetworkHallCall(msg network.AssignmentMessage, orderStatusChan chan network.OrderStatusMessage, txAckChan chan network.AckMessage) {
	if msg.TargetID != config.LocalID {
        return
    }
	recentMessagesMutex.Lock()
    if _, exists := recentAssignments[msg.SeqNum]; exists {
        fmt.Printf("[Duplicate Detected - recentAssignments] Ignoring duplicate assignment: Floor %d, Button %v | SeqNum: %d\n", msg.Floor, msg.Button, msg.SeqNum)
        recentMessagesMutex.Unlock()
        return
    }
    recentAssignments[msg.SeqNum] = time.Now()
    recentMessagesMutex.Unlock()
	fmt.Printf("Received hall assignment for me from network: Floor %d, Button %v\n\n", msg.Floor, msg.Button)
	handleAssignedHallCall(elevio.ButtonEvent{Floor: msg.Floor, Button: msg.Button}, orderStatusChan)

	ackMsg := network.AckMessage{TargetID: config.MasterID, SeqNum: msg.SeqNum}
	fmt.Printf("Broadcasting ack for assignment | SeqNum: %d\n\n", ackMsg.SeqNum)
	for i := 0; i < 3; i++ {
		txAckChan <- ackMsg
		time.Sleep(20 * time.Millisecond)
	}
}

// -----------------------------------------------------------------------------
// Receiving Light Orders
// -----------------------------------------------------------------------------
func handleLightOrder(lightOrder network.LightOrderMessage, txAckChan chan network.AckMessage) {
    if lightOrder.TargetID != config.LocalID {
        return
    }
	recentMessagesMutex.Lock()
    if _, exists := recentLightOrderMessages[lightOrder.SeqNum]; exists {
        fmt.Printf("[Duplicate Detected] Ignoring duplicate Light Order | SeqNum: %d\n", lightOrder.SeqNum)
        recentMessagesMutex.Unlock()
		return
    }
    recentLightOrderMessages[lightOrder.SeqNum] = time.Now()
    recentMessagesMutex.Unlock()

    // Update the button lamp according to the received order
    if lightOrder.Light == network.Off {
        elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, false)
        fmt.Printf("Turned OFF light: Floor %d, Button %v\n", lightOrder.ButtonEvent.Floor, lightOrder.ButtonEvent.Button)
    } else {
        elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, true)
        fmt.Printf("Turned ON light: Floor %d, Button %v\n", lightOrder.ButtonEvent.Floor, lightOrder.ButtonEvent.Button)
    }
    // Send acknowledgment if this elevator is not the Master
    if config.LocalID != config.MasterID {
        ackMsg := network.AckMessage{TargetID: config.MasterID, SeqNum: lightOrder.SeqNum}
        for i := 0; i < 10; i++ {
            txAckChan <- ackMsg
            time.Sleep(10 * time.Millisecond)
        }
        fmt.Printf("Sending ack for LightOrder to master: %s | SeqNum: %d\n", config.MasterID, ackMsg.SeqNum)
    }
}

// -----------------------------------------------------------------------------
// Receiving Orders Status Messages (Only for master)
// -----------------------------------------------------------------------------
func handleOrderStatus(status network.OrderStatusMessage, txAckChan chan network.AckMessage) {
    if config.MasterID != config.LocalID {
        return  // Only the master should process OrderStatusMessages
    }
	recentMessagesMutex.Lock()
    if _, exists := recentOrderStatusMessages[status.SeqNum]; exists {
        fmt.Printf("[Duplicate Detected] Ignoring duplicate Order Status | SeqNum: %d\n", status.SeqNum)
        recentMessagesMutex.Unlock()
		return
    }
    recentOrderStatusMessages[status.SeqNum] = time.Now()
	recentMessagesMutex.Unlock()

    // Send acknowledgment
	ackMsg := network.AckMessage{TargetID: config.MasterID, SeqNum: status.SeqNum}
	for i := 0; i < 10; i++ {
		txAckChan <- ackMsg
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Printf("Sending ack for OrderStatusMessage to: %s | SeqNum: %d\n", status.SenderID, status.SeqNum)
	
    // Process the status message
    if status.Status == network.Unfinished {
        fmt.Printf("Received unfinished order status from elevator %s\n", status.SenderID)
        elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, true)
		network.SendLightOrder(status.ButtonEvent, network.On, status.SenderID)
		fmt.Printf("Turned on order hall light for all elevators\n\n")
    } else if status.Status == network.Finished {
        fmt.Printf("Received finished order status from elevator %s\n", status.SenderID)
        elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, false)
		network.SendLightOrder(status.ButtonEvent, network.Off, status.SenderID)
		fmt.Printf("Turned off order hall light for all elevators\n\n")
    }
}

func flushRecentMessages() {
    for {
        time.Sleep(10 * time.Second) 
        now := time.Now()
        
        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentAssignments {
            if now.Sub(timestamp) > config.MessageTimeout {
                delete(recentAssignments, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentRawHallCalls {
            if now.Sub(timestamp) > config.MessageTimeout {
                delete(recentRawHallCalls, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentOrderStatusMessages {
            if now.Sub(timestamp) > config.MessageTimeout {
                delete(recentOrderStatusMessages, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentLightOrderMessages {
            if now.Sub(timestamp) > config.MessageTimeout {
                delete(recentLightOrderMessages, seqNum)
            }
        }
        recentMessagesMutex.Unlock()
    }
}

// Remove Assignment from recentAssignments when finished
func MarkAssignmentAsCompleted(seqNum int) {
    recentMessagesMutex.Lock()
    delete(recentAssignments, seqNum)
    recentMessagesMutex.Unlock()
}
