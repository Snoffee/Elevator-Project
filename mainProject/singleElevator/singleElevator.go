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
	movementTimer = time.NewTimer(20 * time.Second)
	obstructionTimer = time.NewTimer(20 * time.Second)
	doorTimer = time.NewTimer(config.DoorOpenTime * time.Second)
	clearOppositeDirectionTimer = time.NewTimer(config.DoorOpenTime * time.Second)
	delayedButtonEvent elevio.ButtonEvent // Store delayed call for later clearance
)

// **Run Single Elevator Logic**
func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {
	
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
	

	fmt.Println("Single Elevator Module Running...")

	// Start the receiver to listen for hall assignments
	assignedNetworkHallCallChan := make(chan network.AssignmentMessage, 10) 
	go bcast.Receiver(30002, assignedNetworkHallCallChan) // hallCallPort

	// Create a channel to receive raw hall calls.
	rawHallCallChan := make(chan network.RawHallCallMessage, 10)
	go bcast.Receiver(30003, rawHallCallChan) // rawHallCallPort

	// Start the receiver to listen for light orders
	lightOrderChan := make(chan network.LightOrderMessage, 10)
	go bcast.Receiver(30006, lightOrderChan) // lightPort

	rawHallCallChan2 := make(chan network.RawHallCallMessage, 10)
	go bcast.Transmitter(30004, rawHallCallChan2) // rawHallCallPort



	// Event Loop
	for {
		// Prioritize floor sensor events
		select {
		case floorEvent := <-floorSensor:
			ProcessFloorArrival(floorEvent, orderStatusChan) 
		
		case buttonEvent := <-buttonPress:
			ProcessButtonPress(buttonEvent, hallCallChan, orderStatusChan) // Handle button press event

		case obstructionEvent := <-obstructionSwitch:
			ProcessObstruction(obstructionEvent) // Handle obstruction event
		
		// Hall calls
		case assignedOrder := <-assignedHallCallChan:
			handleAssignedHallCall(assignedOrder, orderStatusChan) // Handle local assigned hall call
		
		case rawCall := <-rawHallCallChan:
			handleAssignedRawHallCall(rawCall, hallCallChan, rawHallCallChan2) // Handle global assigned hall call
		
		case networkAssignedOrder := <-assignedNetworkHallCallChan:
			handleAssignedNetworkHallCall(networkAssignedOrder, orderStatusChan) // Handle network assigned hall call

		case lightOrder := <-lightOrderChan:
			if lightOrder.TargetID == config.LocalID {
				if lightOrder.Light == network.Off {
					elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, false)
				} else {
					elevio.SetButtonLamp(lightOrder.ButtonEvent.Button, lightOrder.ButtonEvent.Floor, true)
				}
			}
		case <- movementTimer.C:
			//stop the elevator due to timeout
			forceShutdown("power loss")

		case <- obstructionTimer.C:
			//stop the elevator due to timeout
			forceShutdown("obstructed too long")
		
		case <- doorTimer.C:
			//close the door
			if !elevator.Obstructed {
				fmt.Println("Transitioning from DoorOpen to Idle...")
				elevio.SetDoorOpenLamp(false)
				elevator.State = config.Idle
				HandleStateTransition()
			}else{
				HandleStateTransition()
			}
		case <- clearOppositeDirectionTimer.C:
			//clear the opposite direction
			fmt.Printf("Clearing delayed opposite direction call: Floor %d, Button %v\n", delayedButtonEvent.Floor, delayedButtonEvent.Button)
			elevio.SetButtonLamp(delayedButtonEvent.Button, delayedButtonEvent.Floor, false)
			elevator.Queue[delayedButtonEvent.Floor][delayedButtonEvent.Button] = false
			msg := network.OrderStatusMessage{ButtonEvent: delayedButtonEvent, SenderID: config.LocalID, Status: network.Finished}
			orderStatusChan <- msg
			network.SendOrderStatus(msg)
		}
		network.BroadcastElevatorStatus(GetElevatorState())
		//time.Sleep(100 * time.Millisecond)
	}
}
