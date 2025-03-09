package order_assignment

import (
	"Main_project/single_elevator"
	"Main_project/config"
	"Main_project/elevio"
)

const TravelTime = 10

func cost(elevator config.Elevator, order elevio.ButtonEvent) int {
	//TODO: check if available, if not assign high cost
	var e config.Elevator
	e = elevator
	e.Queue[order.Floor][order.Button] = true

	timeToCompleteOrders := 0

	//switch with current state of elevator, initial time assessment
	switch {
	case e.State == config.Idle:
		if single_elevator.ChooseDirection(elevator) == elevio.MD_Stop {
			return timeToCompleteOrders
		}
	case e.State == config.Moving:
		timeToCompleteOrders += (TravelTime / 2) //assumes halfway to next floor
		e.Floor += int(e.Direction)
	case e.State == config.DoorOpen:
		timeToCompleteOrders += (config.DoorOpenTime / 2)
	}
	for { //loop that simulates all orders being completed before returning simulated time
		if shouldStopAtFloor(e) {
			clearCurrentFloorFromQueue(e)
			timeToCompleteOrders += config.DoorOpenTime
			single_elevator.ChooseDirection(e)
			if e.Direction == elevio.MD_Stop {
				return timeToCompleteOrders
			}
		}
		e.Floor += int(e.Direction)
		timeToCompleteOrders += TravelTime
	}

}

func shouldStopAtFloor(elevator config.Elevator) bool {
	switch {
	case elevator.Direction == elevio.MD_Up:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] == true ||
			elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] == true ||
			!single_elevator.HasOrdersAbove(elevator)
	case elevator.Direction == elevio.MD_Down:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] == true ||
			elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] == true ||
			!single_elevator.HasOrdersBelow(elevator)
	default:
		return true
	}
}

func clearCurrentFloorFromQueue(elevator config.Elevator) {
	elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] = false
	switch {
	case elevator.Direction == elevio.MD_Up:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		if !single_elevator.HasOrdersAbove(elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		}
	case elevator.Direction == elevio.MD_Down:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		if !single_elevator.HasOrdersBelow(elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		}
	}
}
