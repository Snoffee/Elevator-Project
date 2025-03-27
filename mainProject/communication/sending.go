package communication

import (
	"fmt"
	"mainProject/config"
	"mainProject/elevio"
	"time"
)

// -----------------------------------------------------------------------------
// Assignment and Hall Call Management
// -----------------------------------------------------------------------------
// Sends an assignment message to a specific elevator for a hall call.
func SendAssignment(targetElevator string, floor int, button elevio.ButtonType) {	
	seqNumAssignmentCounter++
	hallCall := AssignmentMessage{
		TargetID: targetElevator,
		Floor:    floor,
		Button:   button,
		SeqNum:   seqNumAssignmentCounter,
	}
	go reliablePacketTransmit(hallCall, txAssignmentChan, hallCall.SeqNum, targetElevator, "Assignment Message", MessageRedundancyFactor)
}
// Sends a raw hall call event to the master elevator for assignment.
func SendRawHallCall(hallCall elevio.ButtonEvent) {
    if config.LocalID == config.MasterID {
        return
    }
    seqNumRawCallCounter++
    msg := RawHallCallMessage{
		TargetID: config.MasterID, 
		SenderID: config.LocalID, 
		Floor: 	  hallCall.Floor, 
		Button:	  hallCall.Button, 
		SeqNum:	  seqNumRawCallCounter,
	}
	go reliablePacketTransmit(msg, txRawHallCallChan, msg.SeqNum, config.MasterID, "Raw Hall Call", MessageRedundancyFactor)
}

// -----------------------------------------------------------------------------
// Light and Order Status Management
// -----------------------------------------------------------------------------
func SendOrderStatus(msg OrderStatusMessage) {
	SeqOrderStatusCounter++
	msg.SeqNum = SeqOrderStatusCounter

	var redundancyFactor int
	if msg.Status == Finished {
		redundancyFactor = MessageRedundancyFactor   // Send Finished messages with higher redundancy
	} else {
		redundancyFactor = MessageRedundancyFactor
	}
	go reliablePacketTransmit(msg, txOrderStatusChan, msg.SeqNum, config.MasterID, "Order Status Message", redundancyFactor)
}

func SendLightOrder(buttonLight elevio.ButtonEvent, lightOnOrOff LightStatus, statusSenderID string) {
	for _, elevator := range elevatorStatuses {
		if elevator.ID == config.LocalID || elevator.ID == statusSenderID {
			continue
		}
		seqLightCounter++
		msg := LightOrderMessage{
			TargetID:    elevator.ID,
			ButtonEvent: buttonLight,
			Light:       lightOnOrOff,
			SeqNum:      seqLightCounter,
		}
		go reliablePacketTransmit(msg, txLightChan, msg.SeqNum, msg.TargetID, "Light Order", MessageRedundancyFactor)
	}
}

// -----------------------------------------------------------------------------
// Combined Message Handling
// -----------------------------------------------------------------------------
func reliablePacketTransmit(msg interface{}, txChan interface{}, seqNum int, targetID string, description string, redundancyFactor int) {
    ackChan := make(chan struct{})
    pendingAcksMutex.Lock()
    pendingAcks[seqNum] = ackChan
    pendingAcksMutex.Unlock()

    retries := 0
    currentInterval := MessageRetryInterval

    for retries < MessageMaxRetries {
        for i := 0; i < redundancyFactor; i++ {
            switch ch := txChan.(type) { 
            case chan AssignmentMessage:
                ch <- msg.(AssignmentMessage)
            case chan RawHallCallMessage:
                ch <- msg.(RawHallCallMessage)
            case chan OrderStatusMessage:
                ch <- msg.(OrderStatusMessage)
            case chan LightOrderMessage:
                ch <- msg.(LightOrderMessage)
            }
        }

        select {
        case <-ackChan:
            fmt.Printf("[ACK Received] %s | SeqNum: %d | Target: %s\n", description, seqNum, targetID)
			return
        case <-time.After(currentInterval):
            retries++
            currentInterval *= time.Duration(MessageExponentialBackoff)
            fmt.Printf("[Retrying] %s | SeqNum: %d | Attempt: %d/%d\n", description, seqNum, retries, MessageMaxRetries)
        }
    }
    fmt.Printf("[Failed] %s | SeqNum: %d | Could not be delivered after %d attempts.\n", description, seqNum, MessageMaxRetries)
    pendingAcksMutex.Lock()
    delete(pendingAcks, seqNum)
    pendingAcksMutex.Unlock()
}