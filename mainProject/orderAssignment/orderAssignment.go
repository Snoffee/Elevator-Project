package orderAssignment

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/masterElection"
	"mainProject/communication"
	"fmt"
)

// Run Order Assignment as a Goroutine
func RunOrderAssignment(elevatorStatusesChan chan map[string]communication.ElevatorStatus, masterChan chan string, lostPeerChan chan string, newPeerChan chan string, hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan communication.OrderStatusMessage, txAckChan chan communication.AckMessage) {

	go func() {
		var latestElevatorStatuses map[string]communication.ElevatorStatus

		for {
			select {
			case updatedStatuses := <-elevatorStatusesChan:
				latestElevatorStatuses = updatedStatuses 

			case newMaster := <-masterChan:
				config.MasterID = newMaster 

			case lostElevator := <-lostPeerChan:
				if lostElevator == config.MasterID {
					masterElection.ElectMaster(latestElevatorStatuses, masterChan)
					newMasterID := <-masterChan
					config.MasterID = newMasterID
				}
				if config.MasterID == config.LocalID && latestElevatorStatuses != nil {
					reassignedHallOrders := getReassignedHallOrders(lostElevator, latestElevatorStatuses)
					for _, order := range reassignedHallOrders {
						bestElevator := findBestElevator(order.Floor, latestElevatorStatuses, lostElevator) 
						fmt.Printf("Reassigned order at floor %d to %s\n\n", order.Floor, bestElevator)
						if bestElevator == config.LocalID {
							assignedHallCallChan <- order
						} else {
							communication.SendAssignment(bestElevator, order.Floor, order.Button)
						}
					}
				}
			case newElevator := <-newPeerChan:
				if config.MasterID == config.LocalID && latestElevatorStatuses != nil {
					backupStates := communication.GetBackupState()
					reassignCabCalls := getReassignedCabCalls(newElevator, backupStates)
					for _, call := range reassignCabCalls {
						fmt.Printf("Reassigning cab call at floor %d to %s\n\n", call.Floor, newElevator)
						communication.SendAssignment(newElevator, call.Floor, call.Button)
					}
				}
			case hallCall := <-hallCallChan: 
				if config.MasterID == config.LocalID {
					bestElevator := findBestElevator(hallCall.Floor, latestElevatorStatuses, "") // Passing "" on excludeElevator when normally calling AssignHallOrder		
					
					if bestElevator == config.LocalID {
						assignedHallCallChan <- hallCall
						fmt.Printf("Assigned hall call to local elevator at floor %d\n\n", hallCall.Floor)
					} else {
						go communication.SendAssignment(bestElevator, hallCall.Floor, hallCall.Button)
						fmt.Printf("Sent hall assignment to elevator: %s\n\n", bestElevator)
					}
				} else {
					go communication.SendRawHallCall(hallCall)
					fmt.Printf("Forwarded hall call to master: %s\n\n", config.MasterID)
				}
			}
		}
	}()
}

// Reassign hall orders if an elevator disconnects
func getReassignedHallOrders(lostElevator string, elevatorStatuses map[string]communication.ElevatorStatus) []elevio.ButtonEvent{
	reassignedOrders := []elevio.ButtonEvent{}
	state, exists := elevatorStatuses[lostElevator];
	if !exists {
		return reassignedOrders
	}
	fmt.Printf("Reassigning hall calls from elevator %s...\n", lostElevator)

	for floor := 0; floor < config.NumFloors; floor++ {
		for button := 0; button < config.NumButtons; button++ {
			if button == int(elevio.BT_Cab) || !state.Queue[floor][button] {
				continue
			}
			reassignedOrders = append(reassignedOrders, elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)})
		}
	}
	return reassignedOrders
}

// Send cab calls back to a recovering elevator
func getReassignedCabCalls(recoveredElevator string, backupElevatorStates map[string]communication.ElevatorStatus) []elevio.ButtonEvent {
	reassignedCabCalls := []elevio.ButtonEvent{}
	state, exists := backupElevatorStates[recoveredElevator]; 
	if !exists {
		return reassignedCabCalls
	} 	
	fmt.Printf("Restoring state: %v\n", state.Queue)

	for floor := 0; floor < config.NumFloors; floor++ {
		if state.Queue[floor][elevio.BT_Cab] {
			reassignedCabCalls = append(reassignedCabCalls, elevio.ButtonEvent{Floor: floor, Button: elevio.BT_Cab})
		}
	}
	return reassignedCabCalls
}

// Determines the closest available elevator to a given floor and button call
func findBestElevator(floor int, elevatorStatuses map[string]communication.ElevatorStatus, excludeElevator string) string {
	fmt.Printf("Available elevators: %v\n\n", elevatorStatuses)
	bestElevator := ""
	bestDistance := config.NumFloors + 1

	for id, state := range elevatorStatuses {
		if id == excludeElevator { 
			continue 
		}
		distance := abs(state.Floor - floor)
		fmt.Printf("Checking elevator %s at floor %d (distance: %d)\n", id, state.Floor, distance)

		if distance < bestDistance {
			bestElevator = id
			bestDistance = distance
		}
	}
	fmt.Println()
	return bestElevator
}

// Helper function to calculate absolute distance
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

