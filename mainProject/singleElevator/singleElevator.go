package singleElevator

import (
	"mainProject/elevio"
	"mainProject/communication"
	"mainProject/network/bcast"
	"mainProject/config"
	"fmt"
	"time"
)

const (
	notMovingTimeLimit = 8 // Seconds
	obstructionTimeLimit = 4 // Seconds
)

var (
	movementTimer				= time.NewTimer(notMovingTimeLimit * time.Second)
	obstructionTimer 			= time.NewTimer(obstructionTimeLimit * time.Second)
	doorTimer 					= time.NewTimer(config.DoorOpenTime * time.Second)
	clearOppositeDirectionTimer = time.NewTimer(config.DoorOpenTime * time.Second)
	delayedButtonEvent 			  elevio.ButtonEvent // Store delayed call for later clearance
)

func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan communication.OrderStatusMessage, txAckChan chan communication.AckMessage, localStatusUpdateChan chan config.Elevator) {
	
	//Initial stop of timers, as we do not need them yet
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

	//Start receivers for hall assignments, hall calls and light orders
	assignedNetworkHallCallChan := make(chan communication.AssignmentMessage, 50) 
	go bcast.Receiver(30002, assignedNetworkHallCallChan) // hallCallPort

	rawHallCallChan := make(chan communication.RawHallCallMessage, 50)
	go bcast.Receiver(30003, rawHallCallChan) // rawHallCallPort

	lightOrderChan := make(chan communication.LightOrderMessage, 50)
	go bcast.Receiver(30006, lightOrderChan) // lightPort

	//Start Transmitter for acks
	go bcast.Transmitter(30004, txAckChan) // ackPort

	go flushRecentMessages()

	// Periodic Broadcast - Continuously broadcasts the elevator status to other elevators
    go func() {
        for {
            time.Sleep(1 * time.Second)
			localStatusUpdateChan <- GetElevatorState()        
		}
    }()

	for {
		// I/O events
		select {
		case floorEvent := <-floorSensor:
			ProcessFloorArrival(floorEvent, orderStatusChan, localStatusUpdateChan) 
		
		case buttonEvent := <-buttonPress:
			ProcessButtonPress(buttonEvent, hallCallChan, orderStatusChan, localStatusUpdateChan) 

		case obstructionEvent := <-obstructionSwitch:
			ProcessObstruction(obstructionEvent, orderStatusChan) 
		
		// Hall calls
		case assignedOrder := <-assignedHallCallChan:
			handleAssignedHallCall(assignedOrder, orderStatusChan, localStatusUpdateChan) 
		
		case rawCall := <-rawHallCallChan:
			handleAssignedRawHallCall(rawCall, hallCallChan, txAckChan) 
		
		case networkAssignedOrder := <-assignedNetworkHallCallChan:
			handleAssignedNetworkHallCall(networkAssignedOrder, orderStatusChan, txAckChan, localStatusUpdateChan) 
		
		// Light orders
		case lightOrder := <-lightOrderChan:
			handleLightOrder(lightOrder, txAckChan)
		
		// Order status
		case status := <- orderStatusChan:
			handleOrderStatus(status, txAckChan)

		// Timers
		case <- movementTimer.C:
			forceShutdown("Power Loss")

		case <- obstructionTimer.C:
			forceShutdown("Obstructed too Long")
		
		case <- doorTimer.C:
			if !elevator.Obstructed {
				fmt.Println("Transitioning from DoorOpen to Idle...")
				elevio.SetDoorOpenLamp(false)
				firstClearButton, secondClearButton, shouldDelaySecondClear := hallCallClearOrder(elevator.Floor)
				clearAllOrdersAtFloor(elevator.Floor, orderStatusChan, localStatusUpdateChan, firstClearButton)
				if shouldDelaySecondClear{
					fmt.Println("Keeping door open for an extra 3 seconds before changing direction...")
					movementTimer.Stop()
					clearOppositeDirectionTimer.Reset(config.DoorOpenTime * time.Second)
					elevio.SetDoorOpenLamp(true)
					delayedButtonEvent = elevio.ButtonEvent{Button: secondClearButton, Floor: elevator.Floor}
				}else{
				elevator.State = config.Idle
				HandleStateTransition(orderStatusChan)
				}
			}else{
				HandleStateTransition(orderStatusChan)
			}

		case <- clearOppositeDirectionTimer.C:
			fmt.Printf("Clearing delayed opposite direction call: Floor %d, Button %v\n", delayedButtonEvent.Floor, delayedButtonEvent.Button)
			elevio.SetDoorOpenLamp(false)
			elevio.SetButtonLamp(delayedButtonEvent.Button, delayedButtonEvent.Floor, false)
			elevator.Queue[delayedButtonEvent.Floor][delayedButtonEvent.Button] = false

			//Send finished order status message to sync hall button lights
			msg := communication.OrderStatusMessage{ButtonEvent: delayedButtonEvent, SenderID: config.LocalID, Status: communication.Finished}
			communication.SendOrderStatus(msg, orderStatusChan)
			MarkAssignmentAsCompleted(msg.SeqNum)
			elevator.State = config.Idle
			HandleStateTransition(orderStatusChan)
		}
		localStatusUpdateChan <- GetElevatorState()	
	}
}
