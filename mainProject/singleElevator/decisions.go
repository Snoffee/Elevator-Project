package singleElevator

import (
	"mainProject/config"
	"mainProject/elevio"
)

// Decides next direction
func ChooseDirection(e config.Elevator) elevio.MotorDirection {
	switch e.Direction {
	case elevio.MD_Up:
		if HasOrdersAbove(e) {
			return elevio.MD_Up
		} else if HasOrdersBelow(e) {
			return elevio.MD_Down
		} else {
			return elevio.MD_Stop
		}
	case elevio.MD_Stop:
		fallthrough

	case elevio.MD_Down:
		if HasOrdersBelow(e) {
			return elevio.MD_Down
		} else if HasOrdersAbove(e) {
			return elevio.MD_Up
		} else {
			return elevio.MD_Stop
		}
	}
	return elevio.MD_Stop
}

func HasOrdersAbove(e config.Elevator) bool {
	for f := e.Floor + 1; f < config.NumFloors; f++ {
		for b := 0; b < config.NumButtons; b++ {
			if e.Queue[f][b] {
				return true
			}
		}
	}
	return false
}

func HasOrdersBelow(e config.Elevator) bool {
	for f := 0; f < e.Floor; f++ {
		for b := 0; b < config.NumButtons; b++ {
			if e.Queue[f][b] {
				return true
			}
		}
	}
	return false
}

// Checks if there are orders at the floor
func hasOrdersAtFloor(floor int) bool {
	return elevator.Queue[floor] != [config.NumButtons]bool{false}
}