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
var stopTimeout chan bool

// **Get the entire elevator state**
func GetElevatorState() config.Elevator {
	return elevator
}

// **Set the entire elevator state (for testing purposes)**
func SetElevatorState(e config.Elevator) {
	elevator = e
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
		for elevio.GetFloor() == -1{
			elevio.SetMotorDirection(elevio.MD_Down)
		}
		elevio.SetMotorDirection(elevio.MD_Stop)
		fmt.Printf("My motordirection is: %v\n", elevator.Direction)
		elevator.Floor = elevio.GetFloor()
		
	default:
		elevator.Floor = elevio.GetFloor()
	}
	elevio.SetFloorIndicator(elevator.Floor)
	fmt.Printf("I'm starting at floor %v\n", elevator.Floor)
}

// **Handles state transitions**
func HandleStateTransition() {
	fmt.Printf("Handling state transition from %v\n", elevator.State)

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

			// Cancel previous timeout and start a new one
			if stopTimeout != nil {
				stopTimeout <- true
			}
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
func startTimeout(e config.Elevator) {
	stopTimeout = make(chan bool, 1) // Reset timeout channel
	timeLimit := time.Duration(config.DestinationTimeLimit) * time.Second
	select {
	case <-time.After(timeLimit):
		if e.State == config.Moving && elevio.GetFloor() != e.Destination {
			fmt.Printf("Power loss detected. Failed to reach floor %d from floor %d\n", e.Destination, e.Floor)
			forceShutdown("power loss")
		} 
	case <-stopTimeout:
		return // Cancel timeout if a movment starts
	}
}

// Variable to hold the forceShutdown function, allowing it to be mocked during testing
var forceShutdown = func(reason string) {
	fmt.Printf("Forcefully shutting down the system due to: %s\n", reason)
	os.Exit(1)
}


