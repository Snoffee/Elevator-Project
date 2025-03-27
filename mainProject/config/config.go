package config

import (
	"mainProject/elevio"
	"os"
	"fmt"
	"time"
	"math/rand"
)

type ElevatorState int

const (
	Idle ElevatorState = iota
	Moving
	DoorOpen
)

type Elevator struct {
	Floor       int
	Direction   elevio.MotorDirection
	Queue       [NumFloors][NumButtons]bool
	State       ElevatorState
	Obstructed  bool
}

const (
	NumFloors  = 4
	NumButtons = 3
	DoorOpenTime = 3 // Seconds
	NotMovingTimeLimit = 8 // Seconds
	ObstructionTimeLimit = 4 // Seconds
	MessageTimeout = 10 * time.Second
)

var LocalID string
var MasterID string

// Initialize LocalID based on hostname
func InitConfig() {
	port := os.Getenv("ELEVATOR_PORT")
	if port == "" {
    	port = "15657" // Default
	}
	elevio.Init("localhost:" + port, NumFloors)

	// Allow for multiple elevators on the same machine
	if id := os.Getenv("ELEVATOR_ID"); id != "" {
		LocalID = id
	} else {
		// Add random number to LocalID to avoid conflicts
		rand.New(rand.NewSource(time.Now().UnixNano()))
		LocalID = fmt.Sprintf("%s_%d", LocalID, rand.Intn(1000))
	}
	fmt.Printf("This elevator's ID: %s\n", LocalID)
}