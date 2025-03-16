// In:
//		Elevator state updates (from `handler.go` when a floor is reached).
//		Obstruction status (from `handler.go` when obstruction event occurs).
//
// Out:
//		GetElevatorState() → Returns elevator's current state to other modules.
//		HandleStateTransition() → Determines the next action for the elevator.

package single_elevator

import (
	"Main_project/config"
	"Main_project/elevio"
	"time"
	"fmt"
	"os"
)

var elevator config.Elevator

// **Get the entire elevator state**
func GetElevatorState() config.Elevator {
	return elevator
}

// **Initialize Elevator**
func InitElevator() {

	elevator = config.Elevator{
		Floor:      0,
		Direction:  elevio.MD_Stop,
		State:      config.Idle,
		Obstructed: false,
		Queue:      [config.NumFloors][config.NumButtons]bool{}, 
	}
	for f := 0; f < config.NumFloors; f++ {
		for b := 0; b < config.NumButtons; b++ {
			button := elevio.ButtonType(b)
			elevio.SetButtonLamp(button, f, false)
		}
	}
	//Correctly sets current floor
	floor := elevio.GetFloor()
	fmt.Printf("Read initial floor as %v\n", floor)
	switch floor{
	case -1:
		for elevio.GetFloor() != 0{
			elevio.SetMotorDirection(elevio.MD_Down)
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
		fmt.Printf("My motordirection is: %v\n", elevator.Direction)
		elevator.Floor = 0
		
	default:
		elevator.Floor = elevio.GetFloor()
	}
	fmt.Printf("I'm starting at floor %v\n", elevator.Floor)
}

// **Handles state transitions**
func HandleStateTransition() {
	fmt.Printf("Handling state transition from %v\n", elevator.State)

	// Safeguard check
	if elevator.Floor < 0 || elevator.Floor >= config.NumFloors - 1 {
		forceShutdown(fmt.Sprintf("invalid floor %d", elevator.Floor))
		return
	}

	switch elevator.State {
	case config.Idle:
		nextDir := ChooseDirection(elevator)
		fmt.Printf("ChooseDirection() returned: %v\n", nextDir) 
		if nextDir != elevio.MD_Stop {
			fmt.Println("Transitioning from Idle to Moving...")
			elevator.State = config.Moving
			elevator.Direction = nextDir
			elevio.SetMotorDirection(nextDir)

			// Track destination and start timer
			elevator.Destination = getNextDestination(elevator, nextDir)
			elevator.MoveStartTime = time.Now()

			// Start timeout
			go startTimeout(elevator)
		} else {
			fmt.Println("No pending orders, staying in Idle.")
		}
	case config.Moving:
		fmt.Println("Elevator is moving...")
		elevio.SetMotorDirection(elevator.Direction)
	case config.DoorOpen:
		if elevator.Obstructed {
			fmt.Println("Door remains open due to obstruction.")
			return
		}
		go func() {
			time.Sleep(config.DoorOpenTime * time.Second)
			if !elevator.Obstructed { 
				fmt.Println("Transitioning from DoorOpen to Idle...")
				elevio.SetDoorOpenLamp(false)
				elevator.State = config.Idle
				HandleStateTransition()
			}
		}()
	}
	fmt.Println()
}

// **Start timeout to check if the elevator reaches the destination within a time limit**
func startTimeout(elevator config.Elevator) {
	timeLimit := time.Duration(config.DestinationTimeLimit) * time.Second
	time.Sleep(timeLimit)
	if elevator.State == config.Moving && time.Since(elevator.MoveStartTime) > timeLimit {
		if elevio.GetFloor() != elevator.Destination{
			fmt.Printf("Power loss detected. Failed to reach floor %d from floor %d\n", elevator.Destination, elevator.Floor)
			forceShutdown("power loss")
		} else {
			fmt.Println("Destination reached within time limit.")
		}
	}
}

// Variable to hold the forceShutdown function, allowing it to be mocked during testing
var forceShutdown = func(reason string) {
	fmt.Printf("Forcefully shutting down the system due to: %s\n", reason)
	os.Exit(1)
}


