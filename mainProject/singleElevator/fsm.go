package singleElevator

import (
	"fmt"
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/communication"
	"os"
	"time"
)

var elevator config.Elevator

// Get the entire elevator state
func GetElevatorState() config.Elevator {
	return elevator
}

// Initialize Elevator
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
	elevator.Obstructed = elevio.GetObstruction()
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
	communication.BroadcastElevatorStatus(GetElevatorState(), true)
	fmt.Printf("I'm starting at floor %v\n", elevator.Floor)

	elevator.State = config.DoorOpen
	if elevator.Floor != -1 {
		elevio.SetDoorOpenLamp(true)
		time.Sleep(config.DoorOpenTime * time.Second)
		elevio.SetDoorOpenLamp(false)
	}
}

// Handles state transitions
func HandleStateTransition() {
	fmt.Printf("Handling state transition from %v\n", elevator.State)

	switch elevator.State {
	case config.Idle:
		obstructionTimer.Stop()
		nextDir := ChooseDirection(elevator)
		fmt.Printf("ChooseDirection() returned: %v\n", nextDir) 
		if nextDir != elevio.MD_Stop {
			fmt.Println("Transitioning from Idle to Moving...")
			// Cancel previous timeout and start a new one
			movementTimer.Reset(config.NotMovingTimeLimit * time.Second)
			elevator.State = config.Moving
			elevator.Direction = nextDir
			elevio.SetMotorDirection(nextDir)

		} else {
			fmt.Println("No pending orders, staying in Idle.")
			movementTimer.Stop()
		}
	case config.Moving:
		obstructionTimer.Stop()
		fmt.Println("Elevator is moving...")
		elevio.SetMotorDirection(elevator.Direction)
	case config.DoorOpen:
		movementTimer.Stop()
		if elevator.Obstructed {
			fmt.Println("Door remains open due to obstruction.")
			doorTimer.Stop()
			obstructionTimer.Reset(config.ObstructionTimeLimit * time.Second)
			return
		}
		doorTimer.Reset(config.DoorOpenTime * time.Second)
	fmt.Println()
	}
}

// Forcing shutdown when elevator is in a fault state
func forceShutdown(reason string) {
	fmt.Printf("Forcefully shutting down the system due to: %s\n", reason)
	os.Exit(1)
}