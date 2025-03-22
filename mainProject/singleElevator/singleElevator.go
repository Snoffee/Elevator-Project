package singleElevator

import (
	"mainProject/elevio"
	"mainProject/network"
	"mainProject/network/bcast"
	"mainProject/config"
	"fmt"
	"time"
)
var movementTimer = time.NewTimer(20 * time.Second)
var obstructionTimer = time.NewTimer(20 * time.Second)
	
// **Run Single Elevator Logic**
func RunSingleElevator(hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {
	
	movementTimer.Stop()
	obstructionTimer.Stop()

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
		select {
		// Hardware		
		case buttonEvent := <-buttonPress:
			ProcessButtonPress(buttonEvent, hallCallChan, orderStatusChan) // Handle button press event
		
		case floorEvent := <-floorSensor:
			ProcessFloorArrival(floorEvent, orderStatusChan) // Handle floor sensor event

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
		}
		
		network.BroadcastElevatorStatus(GetElevatorState())
		time.Sleep(300 * time.Millisecond)
	}
}
