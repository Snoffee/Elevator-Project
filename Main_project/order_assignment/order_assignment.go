// In:
//		elevatorStateChan (from network.go) → Assigns orders dynamically.
//		masterChan (from master_election.go) → Ensures only the master assigns orders.
//		lostPeerChan (from peer_monitor.go) → Reassigns orders if an elevator disconnects.
//		hallCallChan (from single_elevator.go) → Receives hall calls from individual elevators.

// Out:
//		assignedHallCallChan (to single_elevator.go) → Sends hall call assignments back to the requesting elevator.
//		SendHallAssignment → BroadcastHallAssignment (to network.go) → Sends hall call assignments to other elevators.

package order_assignment

import (
	"Main_project/config"
	"Main_project/elevio"
	"Main_project/master_election"
	"Main_project/network"
	"fmt"
)

// **Run Order Assignment as a Goroutine**
func RunOrderAssignment(
	elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string, lostPeerChan chan string, newPeerChan chan string, hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage) {

	go func() {
		var latestMasterID string
		var latestElevatorStates map[string]network.ElevatorStatus

		for {
			select {
			case updatedStates := <-elevatorStateChan:
				latestElevatorStates = updatedStates 

			case newMaster := <-masterChan:
				latestMasterID = newMaster 
				fmt.Printf("Updated Master ID: %s\n\n", latestMasterID)

			case lostElevator := <-lostPeerChan:
				if lostElevator == latestMasterID {
					fmt.Printf("Master elevator %s disconnected! Reassigning hall orders...\n\n", lostElevator)
					master_election.ElectMaster(latestElevatorStates, masterChan)
					newMasterID := <-masterChan
					latestMasterID = newMasterID
				}
				if latestMasterID == config.LocalID && latestElevatorStates != nil {
					fmt.Printf("Lost elevator detected: %s. Reassigning hall orders...\n\n", lostElevator)
					ReassignLostHallOrders(lostElevator, latestElevatorStates, assignedHallCallChan)
				}
			case newElevator := <-newPeerChan:
				if latestMasterID == config.LocalID && latestElevatorStates != nil {
					fmt.Printf("New elevator detected: %s. Restoring cab calls...\n\n", newElevator)
					ReassignCabCalls(newElevator)
				}
			case hallCall := <-hallCallChan: 
				if latestMasterID == config.LocalID {
					bestElevator := AssignHallOrder(hallCall.Floor, hallCall.Button, latestElevatorStates, "") // Passing "" on excludeElevator when normally calling AssignHallOrder		
					
					if bestElevator == config.LocalID {
						assignedHallCallChan <- hallCall
						fmt.Printf("Assigned hall call to local elevator at floor %d\n\n", hallCall.Floor)
					} else {
						network.SendAssignment(bestElevator, hallCall.Floor, hallCall.Button)
						fmt.Printf("Sent hall assignment to elevator: %s\n\n", bestElevator)
					}
				} else {
					network.SendRawHallCall(latestMasterID, hallCall)
					fmt.Printf("Forwarded hall call to master: %s\n\n", latestMasterID)
				}
			case status := <-orderStatusChan:
				if latestMasterID == config.LocalID {
					if status.Status == network.Unfinished {
						network.SendLightOrder(status.ButtonEvent, network.On)
						elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, true)
						fmt.Printf("Turned on order hall light for all elevators\n\n")
					} else {
						network.SendLightOrder(status.ButtonEvent, network.Off)
						elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, false)
						fmt.Printf("Turned off order hall light for all elevators\n\n")
					}
				}
		}
	}
	}()
}

// **Reassign hall orders if an elevator disconnects**
func ReassignLostHallOrders(lostElevator string, elevatorStates map[string]network.ElevatorStatus, assignedHallCallChan chan elevio.ButtonEvent) {
	state, exists := elevatorStates[lostElevator];
	if !exists {
		fmt.Printf("Lost elevator %s not found in state map!\n", lostElevator)
		return
	}

	fmt.Printf("Reassigning hall calls from elevator %s...\n", lostElevator)

	for floor := 0; floor < config.NumFloors; floor++ {
		for button := 0; button < config.NumButtons; button++ {
			if button == int(elevio.BT_Cab) || !state.Queue[floor][button] {
				continue
			}
			bestElevator := AssignHallOrder(floor, elevio.ButtonType(button), elevatorStates, lostElevator) // Reassign order

			if bestElevator == "" {
				fmt.Printf("No available elevator for reassignment at floor %d\n\n", floor)
				continue
			}
			
			fmt.Printf("Reassigned order at floor %d to %s\n\n", floor, bestElevator)
			
			if bestElevator == config.LocalID {
				assignedHallCallChan <- elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)}
			} else {
				network.SendAssignment(bestElevator, floor, elevio.ButtonType(button))
			}
		}
	}
}

// **Send cab calls back to a recovering elevator**
func ReassignCabCalls(recoveredElevator string) {
	backupElevatorStates := network.GetBackupState()
	state, exists := backupElevatorStates[recoveredElevator]; 
	if !exists {
		fmt.Printf("Recovered elevator %s not found in state map!\n", recoveredElevator)
		return
	} 
	
	fmt.Printf("Restoring state: %v\n", state.Queue)

	for floor := 0; floor < config.NumFloors; floor++ {
		if state.Queue[floor][elevio.BT_Cab] {
			fmt.Printf("Reassigning cab call at floor %d to %s\n\n", floor, recoveredElevator)
			network.SendAssignment(recoveredElevator, floor, elevio.BT_Cab)
		}
	}
}

// **Assign hall order to the closest available elevator**
func AssignHallOrder(floor int, button elevio.ButtonType, elevatorStates map[string]network.ElevatorStatus, excludeElevator string) string {
	fmt.Printf("Available elevators: %v\n\n", elevatorStates)

	bestElevator := ""
	bestDistance := config.NumFloors + 1

	// Find the best elevator based on distance
	for id, state := range elevatorStates {
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

// **Helper function to calculate absolute distance**
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

