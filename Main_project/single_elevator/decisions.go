// In:
//		Elevator state (from `fms.go` via GetElevatorState).
//
// Out:
//		ChooseDirection() → Determines the next movement for the elevator.
//		Helper functions (hasOrdersAbove(), hasOrdersBelow()) → Used in ChooseDirection().

package single_elevator

import (
	"Main_project/config"
	"Main_project/elevio"
)

// **Decides next direction**
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

// **Checks if there are orders at the floor**
func hasOrdersAtFloor(floor int) bool {
	return elevator.Queue[floor] != [config.NumButtons]bool{false}
}

// **Get the next destination for the elevator**
func getNextDestination(elevator config.Elevator, direction elevio.MotorDirection) int {
	switch direction {
	case elevio.MD_Up:
		return getHighestOrderAbove(elevator)
	case elevio.MD_Down:
		return getLowestOrderBelow(elevator)
	default:
		return elevator.Floor
	}
}

// **Get the highest order above the current floor**
func getHighestOrderAbove(elevator config.Elevator) int {
	for f := elevator.Floor + 1; f < config.NumFloors; f++ {
		for b := 0; b < config.NumButtons; b++ {
			if elevator.Queue[f][b] {
				return f
			}
		}
	}
	return elevator.Floor
}

// **Get the lowest order below the current floor**
func getLowestOrderBelow(elevator config.Elevator) int {
	for f := elevator.Floor - 1; f >= 0; f-- {
		for b := 0; b < config.NumButtons; b++ {
			if elevator.Queue[f][b] {
				return f
			}
		}
	}
	return elevator.Floor
}

// **Clears orders at a given floor**
func clearFloorOrders(floor int) {
	elevator.Queue[floor] = [config.NumButtons]bool{false}
	for btn := 0; btn < config.NumButtons; btn++ {
		elevio.SetButtonLamp(elevio.ButtonType(btn), floor, false)
	}
}