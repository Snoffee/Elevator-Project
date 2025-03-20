package singleElevator

import (
	"testing"
	"mainProject/config"
	"mainProject/elevio"
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

	// Override the forceShutdown function with mock
	forceShutdown = mockForceShutdown

	// Simulate starting to move
	elevator.State = config.Moving
	elevator.Direction = elevio.MD_Up
	elevator.MoveStartTime = time.Now()

	// Start timeout and check for power loss
	startTimeout(elevator)

	// Wait for the timeout duration plus a buffer to ensure HandlePowerLoss is called
	time.Sleep(config.DestinationTimeLimit*time.Second + 1*time.Second)

	// Check if the system has exited
	if !shutdownCalled {
		t.Errorf("Expected forceShutdown to be called, but it was not")
	}
}

// Mock function to replace forceShutdown during testing
var shutdownCalled bool

func mockForceShutdown(reason string) {
	shutdownCalled = true
}

func TestSafeguardInvalidFloor(t *testing.T) {
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

	// Override the forceShutdown function with mock
	forceShutdown = mockForceShutdown

	// Test invalid floor below 0
	elevator.Floor = -1
	shutdownCalled = false
	HandleStateTransition()
	if !shutdownCalled {
		t.Errorf("Expected forceShutdown to be called for floor %d, but it was not", elevator.Floor)
	}

	// Test invalid floor above NumFloors
	elevator.Floor = config.NumFloors
	shutdownCalled = false
	HandleStateTransition()
	if !shutdownCalled {
		t.Errorf("Expected forceShutdown to be called for floor %d, but it was not", elevator.Floor)
	}
}

func TestValidFloorTransition(t *testing.T) {
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

	// Ensure no shutdown for valid floor
	forceShutdown = mockForceShutdown
	shutdownCalled = false

	// Move to a valid floor and check transition
	elevator.Floor = 1
	HandleStateTransition()
	if shutdownCalled {
		t.Errorf("Did not expect forceShutdown to be called for floor %d", elevator.Floor)
	}
}