package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/communication"
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

// -----------------------------------------------------------------------------
// Handles an assigned hall call from `orderAssignment`
// -----------------------------------------------------------------------------
func handleAssignedHallCall(order elevio.ButtonEvent, orderStatusChan chan communication.OrderStatusMessage, localStatusUpdateChan chan config.Elevator){
	fmt.Printf(" Received assigned hall call: Floor %d, Button %d\n\n", order.Floor, order.Button)

	elevator.Queue[order.Floor][order.Button] = true
	elevio.SetButtonLamp(order.Button, order.Floor, true)

	if order.Button != elevio.BT_Cab {
		communication.SeqOrderStatusCounter++
		msg := communication.OrderStatusMessage{ButtonEvent: order, SenderID: config.LocalID, Status: communication.Unfinished}
		orderStatusChan <- msg
		go communication.SendOrderStatus(msg)
	}
	floorSensorValue := elevio.GetFloor()
	// If the elevator is already at the assigned floor, immediately process it
    if elevator.Floor == order.Floor && floorSensorValue != -1{
        fmt.Println("Already at assigned floor, processing immediately...")
		time.Sleep(3 * time.Second)
		ProcessFloorArrival(elevator.Floor, orderStatusChan, localStatusUpdateChan)
        localStatusUpdateChan <- GetElevatorState()
    } else {
        HandleStateTransition(orderStatusChan)
    }
}

// -----------------------------------------------------------------------------
// Receiving Raw Hall Call from a slave (Only for master)
// -----------------------------------------------------------------------------
func handleAssignedRawHallCall(rawCall communication.RawHallCallMessage, hallCallChan chan elevio.ButtonEvent, txAckChan chan communication.AckMessage) {
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

    ackMsg:= communication.AckMessage{TargetID: rawCall.SenderID, SeqNum: rawCall.SeqNum}
	fmt.Printf("Broadcasting ack for RawHallCall to sender: %s | SeqNum: %d\n\n", ackMsg.TargetID, ackMsg.SeqNum)
	for i := 0; i < 3; i++ {
		txAckChan <- ackMsg
		time.Sleep(20 * time.Millisecond)
	}

	fmt.Printf("Processing raw hall call: Floor %d, Button %d\n", rawCall.Floor, rawCall.Button)
	hallCallChan <- elevio.ButtonEvent{Floor: rawCall.Floor, Button: rawCall.Button}
	
    //MarkRawHallCallAsCompleted(rawCall.SeqNum)
}

// -----------------------------------------------------------------------------
// Receiving Hall Assignments from master (from network)
// -----------------------------------------------------------------------------
// If the best elevator was another elevator on the network the order gets sent here
func handleAssignedNetworkHallCall(msg communication.AssignmentMessage, orderStatusChan chan communication.OrderStatusMessage, txAckChan chan communication.AckMessage, localStatusUpdateChan chan config.Elevator) {
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

	ackMsg := communication.AckMessage{TargetID: config.MasterID, SeqNum: msg.SeqNum}
	fmt.Printf("Broadcasting ack for assignment | SeqNum: %d\n\n", ackMsg.SeqNum)
	for i := 0; i < 3; i++ {
		txAckChan <- ackMsg
		time.Sleep(20 * time.Millisecond)
	}
    handleAssignedHallCall(elevio.ButtonEvent{Floor: msg.Floor, Button: msg.Button}, orderStatusChan, localStatusUpdateChan)
}

// -----------------------------------------------------------------------------
// Receiving Light Orders
// -----------------------------------------------------------------------------
func handleLightOrder(lightOrder communication.LightOrderMessage, txAckChan chan communication.AckMessage) {
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

    // Send acknowledgment if this elevator is not the Master
    if config.LocalID != config.MasterID {
        ackMsg := communication.AckMessage{TargetID: config.MasterID, SeqNum: lightOrder.SeqNum}
        for i := 0; i < 10; i++ {
            txAckChan <- ackMsg
            time.Sleep(10 * time.Millisecond)
        }
        fmt.Printf("Sending ack for LightOrder to master: %s | SeqNum: %d\n", config.MasterID, ackMsg.SeqNum)
    }

    // Update the button lamp according to the received order
    if lightOrder.Light == communication.Off {
        elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, false)
        fmt.Printf("Turned OFF light: Floor %d, Button %v\n", lightOrder.ButtonEvent.Floor, lightOrder.ButtonEvent.Button)
    } else {
        elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, true)
        fmt.Printf("Turned ON light: Floor %d, Button %v\n", lightOrder.ButtonEvent.Floor, lightOrder.ButtonEvent.Button)
    }
    //MarkLightOrderAsCompleted(lightOrder.SeqNum)
}

// -----------------------------------------------------------------------------
// Receiving Orders Status Messages (Only for master)
// -----------------------------------------------------------------------------
func handleOrderStatus(status communication.OrderStatusMessage, txAckChan chan communication.AckMessage) {
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

    // Send acknowledgment. Multiple to handle packet loss better.
	ackMsg := communication.AckMessage{TargetID: config.MasterID, SeqNum: status.SeqNum}
	for i := 0; i < 10; i++ {
		txAckChan <- ackMsg
		time.Sleep(10 * time.Millisecond)
        fmt.Printf("Sending ack for OrderStatusMessage to: %s | SeqNum: %d\n", status.SenderID, status.SeqNum)
	}
	
    // Process the status message
    if status.Status == communication.Unfinished {
        fmt.Printf("Received unfinished order status from elevator %s\n", status.SenderID)
        elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, true)
		communication.SendLightOrder(status.ButtonEvent, communication.On, status.SenderID)
		fmt.Printf("Turned on order hall light for all elevators\n\n")
    } else if status.Status == communication.Finished {
        fmt.Printf("Received finished order status from elevator %s\n", status.SenderID)
        elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, false)
		communication.SendLightOrder(status.ButtonEvent, communication.Off, status.SenderID)
		fmt.Printf("Turned off order hall light for all elevators\n\n")
    }
    //MarkOrderStatusAsCompleted(status.SeqNum)
}

func flushRecentMessages() {
    const messageTimeout = 10 * time.Second
    for {
        time.Sleep(10 * time.Second) 
        now := time.Now()
        
        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentAssignments {
            if now.Sub(timestamp) > messageTimeout {
                delete(recentAssignments, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentRawHallCalls {
            if now.Sub(timestamp) > messageTimeout {
                delete(recentRawHallCalls, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentOrderStatusMessages {
            if now.Sub(timestamp) > messageTimeout {
                delete(recentOrderStatusMessages, seqNum)
            }
        }
        recentMessagesMutex.Unlock()

        recentMessagesMutex.Lock()
        for seqNum, timestamp := range recentLightOrderMessages {
            if now.Sub(timestamp) > messageTimeout {
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
