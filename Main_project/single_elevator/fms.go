package single_elevator

import (
	"Main_project/config"
	"Main_project/elevio"
	"time"
	"fmt"
)

var elevator config.Elevator

// **Returns the current elevator state**
func GetState() config.ElevatorState {
	return elevator.State
}

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
}


