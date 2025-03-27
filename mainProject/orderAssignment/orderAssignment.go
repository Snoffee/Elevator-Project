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
					ReassignLostHallOrders(lostElevator, latestElevatorStatuses, assignedHallCallChan)
				}
			case newElevator := <-newPeerChan:
				if config.MasterID == config.LocalID && latestElevatorStatuses != nil {
					ReassignCabCalls(newElevator)
				}
			case hallCall := <-hallCallChan: 
				if config.MasterID == config.LocalID {
					bestElevator := AssignHallOrder(hallCall.Floor, hallCall.Button, latestElevatorStatuses, "") // Passing "" on excludeElevator when normally calling AssignHallOrder		
					
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
func ReassignLostHallOrders(lostElevator string, elevatorStatuses map[string]communication.ElevatorStatus, assignedHallCallChan chan elevio.ButtonEvent) {
	state, exists := elevatorStatuses[lostElevator];
	if !exists {
		return
	}
	fmt.Printf("Reassigning hall calls from elevator %s...\n", lostElevator)

	for floor := 0; floor < config.NumFloors; floor++ {
		for button := 0; button < config.NumButtons; button++ {
			if button == int(elevio.BT_Cab) || !state.Queue[floor][button] {
				continue
			}
			bestElevator := AssignHallOrder(floor, elevio.ButtonType(button), elevatorStatuses, lostElevator) 
			fmt.Printf("Reassigned order at floor %d to %s\n\n", floor, bestElevator)
			if bestElevator == config.LocalID {
				assignedHallCallChan <- elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)}
			} else {
				communication.SendAssignment(bestElevator, floor, elevio.ButtonType(button))
			}
		}
	}
}

// Send cab calls back to a recovering elevator
func ReassignCabCalls(recoveredElevator string) {
	backupElevatorStates := communication.GetBackupState()
	state, exists := backupElevatorStates[recoveredElevator]; 
	if !exists {
		return
	} 	
	fmt.Printf("Restoring state: %v\n", state.Queue)

	for floor := 0; floor < config.NumFloors; floor++ {
		if state.Queue[floor][elevio.BT_Cab] {
			fmt.Printf("Reassigning cab call at floor %d to %s\n\n", floor, recoveredElevator)
			communication.SendAssignment(recoveredElevator, floor, elevio.BT_Cab)
		}
	}
}

// Assign hall order to the closest available elevator
func AssignHallOrder(floor int, button elevio.ButtonType, elevatorStatuses map[string]communication.ElevatorStatus, excludeElevator string) string {
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

