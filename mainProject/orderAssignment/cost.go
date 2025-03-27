package orderAssignment

import (
	"mainProject/communication"
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/singleElevator"
	"fmt"
)

const(
	travelTime = 8
)

func cost(elevator communication.ElevatorStatus, order elevio.ButtonEvent) int{

	var e config.Elevator
	e.Floor = elevator.Floor
	e.Direction = elevator.Direction
	e.Queue = elevator.Queue
	e.State = elevator.State
	e.Obstructed = false
	e.Queue[order.Floor][order.Button] = true

	fmt.Printf("My state %v", e.State)

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
		timeToCompleteOrders += (travelTime/2) //antar halvveis mellom etasjer
		e.Floor += int(e.Direction)
	case config.DoorOpen:
		timeToCompleteOrders += config.DoorOpenTime/2
	}
	

	for {
		if shouldStopAtFloor(e) {
			clearCurrentFloorFromQueue(&e)
			timeToCompleteOrders += config.DoorOpenTime
		}
	
		e.Direction = singleElevator.ChooseDirection(e)
	
		if e.Direction == elevio.MD_Stop {
			return timeToCompleteOrders
		}
	
		// Only move if we have a direction
		e.Floor += int(e.Direction)
		timeToCompleteOrders += travelTime
	}
	
}





func shouldStopAtFloor(elevator config.Elevator) bool{
	switch elevator.Direction {
	case elevio.MD_Up:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] == true ||
		elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] ==  true ||
		!singleElevator.HasOrdersAbove(elevator)
	case elevio.MD_Down:
		return elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] == true ||
			elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] == true ||
			!singleElevator.HasOrdersBelow(elevator)
	default:
		return true
	}
}

func clearCurrentFloorFromQueue(elevator *config.Elevator){
	elevator.Queue[elevator.Floor][int(elevio.BT_Cab)] = false
	switch elevator.Direction{
	case elevio.MD_Up:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		if !singleElevator.HasOrdersAbove(*elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		}
	case elevio.MD_Down:
		elevator.Queue[elevator.Floor][int(elevio.BT_HallUp)] = false
		if !singleElevator.HasOrdersBelow(*elevator) {
			elevator.Queue[elevator.Floor][int(elevio.BT_HallDown)] = false
		}
	}
}