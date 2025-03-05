package logic

import (
	"elevator_system/config"
	"elevator_system/elevio"
)

// **Decides next direction**
func ChooseDirection(e config.Elevator) elevio.MotorDirection {
	if hasOrdersAbove(e) {
		return elevio.MD_Up
	} else if hasOrdersBelow(e) {
		return elevio.MD_Down
	} else {
		return elevio.MD_Stop
	}
}

func hasOrdersAbove(e config.Elevator) bool {
	for f := e.Floor + 1; f < config.NumFloors; f++ {
		for b := 0; b < config.NumButtons; b++ {
			if e.Queue[f][b] {
				return true
			}
		}
	}
	return false
}

func hasOrdersBelow(e config.Elevator) bool {
	for f := 0; f < e.Floor; f++ {
		for b := 0; b < config.NumButtons; b++ {
			if e.Queue[f][b] {
				return true
			}
		}
	}
	return false
}
