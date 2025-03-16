package single_elevator

import (
	"testing"
	"Main_project/config"
	"Main_project/elevio"
	"time"
	"os"
)

func TestHandlePowerLoss(t *testing.T) {
	port := os.Getenv("ELEVATOR_PORT")
	if port == "" {
    	port = "15657" // Default
	}
	elevio.Init("localhost:" + port, config.NumFloors)

	InitElevator()

	// Initialize the elevator state
	elevator = config.Elevator{
		Floor:         0,
		Direction:     elevio.MD_Stop,
		State:         config.Idle,
		Obstructed:    false,
		Queue:         [config.NumFloors][config.NumButtons]bool{},
		MoveStartTime: time.Now(),
		Destination:   1,
	}

	// Simulate starting to move
	elevator.State = config.Moving
	elevator.Direction = elevio.MD_Up
	elevator.MoveStartTime = time.Now()

	// Start timeout and check for power loss
	startTimeout(elevator)

	// Wait for the timeout duration plus a buffer to ensure HandlePowerLoss is called
	time.Sleep(config.DestinationTimeLimit*time.Second + 1*time.Second)

	// Check if the system has exited
	if elevator.State != config.Idle {
		t.Errorf("Expected elevator state to be Idle after power loss, but got %v", elevator.State)
	}
}