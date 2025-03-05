package config

import (
	"Main_project2/elevio"
	"os"
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
}