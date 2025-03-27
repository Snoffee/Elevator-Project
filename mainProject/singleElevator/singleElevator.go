package singleElevator

import (
	"mainProject/elevio"
	"mainProject/network"
	"mainProject/network/bcast"
	"mainProject/config"
	"fmt"
	"time"
)

var (
	movementTimer				= time.NewTimer(20 * time.Second)
	obstructionTimer 			= time.NewTimer(20 * time.Second)
	doorTimer 					= time.NewTimer(config.DoorOpenTime * time.Second)
	clearOppositeDirectionTimer = time.NewTimer(config.DoorOpenTime * time.Second)
	delayedButtonEvent 			  elevio.ButtonEvent // Store delayed call for later clearance
)

func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage, txAckChan chan network.AckMessage) {
	
	movementTimer.Stop()
	obstructionTimer.Stop()
	doorTimer.Stop()
	clearOppositeDirectionTimer.Stop()

	// Initialize elevator hardware event channels
	buttonPress       := make(chan elevio.ButtonEvent)
	floorSensor       := make(chan int)
	obstructionSwitch := make(chan bool)


	// Start polling hardware for events
	go elevio.PollButtons(buttonPress)
	go elevio.PollFloorSensor(floorSensor)
	go elevio.PollObstructionSwitch(obstructionSwitch)
	

	fmt.Printf("Single Elevator Module Running...\n\n")

	// Start the receiver to listen for hall assignments
	assignedNetworkHallCallChan := make(chan network.AssignmentMessage, 50) 
	go bcast.Receiver(30002, assignedNetworkHallCallChan) // hallCallPort

	// Create a channel to receive raw hall calls.
	rawHallCallChan := make(chan network.RawHallCallMessage, 50)
	go bcast.Receiver(30003, rawHallCallChan) // rawHallCallPort

	// Start the receiver to listen for light orders
	lightOrderChan := make(chan network.LightOrderMessage, 50)
	go bcast.Receiver(30006, lightOrderChan) // lightPort

	go bcast.Transmitter(30004, txAckChan) // ackPort

	go flushRecentMessages()

	// Periodic Broadcast - Continuously broadcasts the elevator status to other elevators
    go func() {
        for {
            time.Sleep(1 * time.Second) 
            network.BroadcastElevatorStatus(GetElevatorState(), false)
        }
    }()

	// Event Loop
	for {
		// Hardware
		select {
		case floorEvent := <-floorSensor:
			ProcessFloorArrival(floorEvent, orderStatusChan) 
		
		case buttonEvent := <-buttonPress:
			ProcessButtonPress(buttonEvent, hallCallChan, orderStatusChan) 

		case obstructionEvent := <-obstructionSwitch:
			ProcessObstruction(obstructionEvent) 
		
		// Hall calls
		case assignedOrder := <-assignedHallCallChan:
			handleAssignedHallCall(assignedOrder, orderStatusChan) 
		
		case rawCall := <-rawHallCallChan:
			handleAssignedRawHallCall(rawCall, hallCallChan, txAckChan) 
		
		case networkAssignedOrder := <-assignedNetworkHallCallChan:
			handleAssignedNetworkHallCall(networkAssignedOrder, orderStatusChan, txAckChan) 
		
		// Light orders
		case lightOrder := <-lightOrderChan:
			handleLightOrder(lightOrder, txAckChan)
		
		// Order status
		case status := <- orderStatusChan:
			handleOrderStatus(status, txAckChan)

		// Timers
		case <- movementTimer.C:
			forceShutdown("power loss")

		case <- obstructionTimer.C:
			forceShutdown("obstructed too long")
		
		case <- doorTimer.C:
			if !elevator.Obstructed {
				fmt.Println("Transitioning from DoorOpen to Idle...")
				elevio.SetDoorOpenLamp(false)
				elevator.State = config.Idle
				HandleStateTransition()
			}else{
				HandleStateTransition()
			}

		case <- clearOppositeDirectionTimer.C:
			fmt.Printf("Clearing delayed opposite direction call: Floor %d, Button %v\n", delayedButtonEvent.Floor, delayedButtonEvent.Button)
			elevio.SetButtonLamp(delayedButtonEvent.Button, delayedButtonEvent.Floor, false)
			elevator.Queue[delayedButtonEvent.Floor][delayedButtonEvent.Button] = false
			msg := network.OrderStatusMessage{ButtonEvent: delayedButtonEvent, SenderID: config.LocalID, Status: network.Finished}
			orderStatusChan <- msg
			network.SendOrderStatus(msg)
			MarkAssignmentAsCompleted(msg.SeqNum)
		}
		network.BroadcastElevatorStatus(GetElevatorState(), true)
	}
}
