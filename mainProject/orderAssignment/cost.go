package orderAssignment

import (
	"mainProject/communication"
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/singleElevator"
)

const(
	travelTime = 3 //Seconds
)

func cost(elevator communication.ElevatorStatus, order elevio.ButtonEvent) int{
	//Making an elevator object from the passed ElevatorStatus argument
	var e config.Elevator
	e.Floor = elevator.Floor
	e.Direction = elevator.Direction
	e.Queue = elevator.Queue
	e.State = elevator.State
	e.Queue[order.Floor][order.Button] = true

	timeToCompleteOrders := 0

	//switch with current state of elevator, initial time assessment
	switch e.State {
	case config.Idle:
		if singleElevator.ChooseDirection(e) == elevio.MD_Stop{
			if order.Floor == e.Floor {
			return config.DoorOpenTime
			}
		}
	case config.Moving:
		timeToCompleteOrders += (travelTime/2) //Assumes elevator is halfway between floors
		e.Floor += int(e.Direction)
	case config.DoorOpen:
		timeToCompleteOrders += config.DoorOpenTime/2 //Assumes door has been open for half the required time
	}
	
	//Simulates the time it takes to complete orders for the given elevator
	for {
		if shouldStopAtFloor(e) {
			clearCurrentFloorFromQueue(&e)
			timeToCompleteOrders += config.DoorOpenTime
		}
	
		e.Direction = singleElevator.ChooseDirection(e)
	
		if e.Direction == elevio.MD_Stop {
			return timeToCompleteOrders
		}
	
		e.Floor += int(e.Direction)
		timeToCompleteOrders += travelTime
	}
	
}

func shouldStopAtFloor(elevator config.Elevator) bool{
	switch elevator.Direction {
	case elevio.MD_Up:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] == true ||
		elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] ==  true ||
		!HasOrdersAbove(elevator)
	case elevio.MD_Down:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] == true ||
			elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] == true ||
			!HasOrdersBelow(elevator)
	default:
		return true
	}
}

func clearCurrentFloorFromQueue(elevator *config.Elevator){
	elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] = false
	switch elevator.Direction{
	case elevio.MD_Up:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		if !HasOrdersAbove(*elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		}
	case elevio.MD_Down:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		if !HasOrdersBelow(*elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		}
	}
}

// Decides which direction is the most sensible to choose next
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