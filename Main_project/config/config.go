package config

import (
	"Main_project/elevio"
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
)

var LocalID string

// Initialize LocalID based on hostname
func InitConfig() {
	hostname, err := os.Hostname()
	if err != nil {
		LocalID = "elevator_unknown"
	} else {
		LocalID = "elevator_" + hostname // Example: "elevator_PC1"
	}
	// Allow for multiple elevators on the same machine
	if id := os.Getenv("ELEVATOR_ID"); id != "" {
		LocalID = id
	} else {
		// Add random number to LocalID to avoid conflicts
		rand.New(rand.NewSource(time.Now().UnixNano()))
		LocalID = fmt.Sprintf("%s_%d", LocalID, rand.Intn(1000))
	}
}