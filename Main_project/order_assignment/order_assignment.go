// In: 	
//		elevatorStateChan (from network.go) → Assigns orders dynamically.
//		masterChan (from master_election.go) → Ensures only the master assigns orders.
//		lostPeerChan (from peer_monitor.go) → Reassigns orders if an elevator disconnects.	
//		hallCallChan (from single_elevator.go) 

// Out:
//		assignedHallCallChan (used by single_elevator.go) → Assigns hall calls to this elevator.
//		SendHallAssignment → BroadcastHallAssignment (to network.go) (used by single_elevator.go) → Assigns hall calls to other elevators on network

package order_assignment

import (
	"Main_project/config"
	"Main_project/network"
	"Main_project/elevio"
	"fmt"
)

// **Run Order Assignment as a Goroutine**
func RunOrderAssignment(
	elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string, lostPeerChan chan string, hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent) {

	go func() {
		var latestMasterID string
		var latestElevatorStates map[string]network.ElevatorStatus

		for {
			select {
			case updatedStates := <-elevatorStateChan:
				latestElevatorStates = updatedStates // Process received elevator states

			case newMaster := <-masterChan:
				latestMasterID = newMaster // Process received master update
				fmt.Printf("Updated Master ID: %s\n", latestMasterID)

			case lostElevator := <-lostPeerChan:
				fmt.Printf("Lost elevator detected: %s. Reassigning orders...\n", lostElevator)
				if latestMasterID == config.LocalID && latestElevatorStates != nil {
					ReassignLostOrders(lostElevator, latestElevatorStates, assignedHallCallChan)
				}
			case hallCall := <-hallCallChan: // Receives a hall call from single_elevator
				if latestMasterID == config.LocalID {
					bestElevator := AssignHallOrder(hallCall.Floor, hallCall.Button, latestElevatorStates)
					
					if bestElevator == config.LocalID {
						// If this elevator was chosen, send it back to `single_elevator`
						fmt.Printf("Assigned hall call to this elevator at floor %d\n", hallCall.Floor)
						assignedHallCallChan <- hallCall
					} else {
						// If another elevator was chosen, send assignment over network
						SendHallAssignment(bestElevator, hallCall)
					}
				}
			}
		}
	}()
}

// **Reassign orders if an elevator disconnects**
func ReassignLostOrders(lostElevator string, elevatorStates map[string]network.ElevatorStatus, assignedHallCallChan chan elevio.ButtonEvent) {
	fmt.Printf("Reassigning hall calls from elevator %s...\n", lostElevator)

	// Ensure elevator exists before proceeding
	if _, exists := elevatorStates[lostElevator]; !exists {
		fmt.Printf("Lost elevator %s not found in state map!\n", lostElevator)
		return
	}

	// Reassign all hall orders assigned to the lost elevator
	for floor := 0; floor < config.NumFloors; floor++ {
		for button := 0; button < config.NumButtons; button++ {
			if state, exists := elevatorStates[lostElevator]; exists && state.Queue[floor][button] {
				fmt.Printf("Reassigning order at floor %d to a new elevator\n", floor)

				bestElevator := AssignHallOrder(floor, elevio.ButtonType(button), elevatorStates) // Reassign order

				if bestElevator != "" {
					fmt.Printf("Order at floor %d successfully reassigned to %s\n", floor, bestElevator)
					if bestElevator == config.LocalID {
						// Send re-assigned order to this elevator
						assignedHallCallChan <- elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)}
					} else {
						// Otherwise, notify the chosen elevator over the network
						SendHallAssignment(bestElevator, elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)})
					}
				} else {
					fmt.Printf("No available elevator for reassignment!\n")
				}
			}
		}
	}
}

// **Assign hall order to the closest available elevator**
func AssignHallOrder(floor int, button elevio.ButtonType, elevatorStates map[string]network.ElevatorStatus) string {
	fmt.Println("Available elevators:", elevatorStates)

	bestElevator := ""
	bestDistance := config.NumFloors + 1

	// Find the best elevator based on distance
	for id, state := range elevatorStates {
		distance := abs(state.Floor - floor)
		fmt.Printf("Checking elevator %s at floor %d (distance: %d)\n", id, state.Floor, distance)

		if distance < bestDistance {
			bestElevator = id
			bestDistance = distance
		}
	}

	// Assign the order to the best elevator
	if bestElevator != "" {
		fmt.Printf("Assigning hall call at floor %d to %s\n", floor, bestElevator)
		SendHallAssignment(bestElevator, elevio.ButtonEvent{Floor: floor, Button: button})
	} else {
		fmt.Println("No available elevator found!")
	}

	return bestElevator
}

// **Helper function to calculate absolute distance**
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// **Send Hall Assignment Over Network**
func SendHallAssignment(elevatorID string, hallCall elevio.ButtonEvent) {
	fmt.Printf(" Sending hall order to elevator %s: Floor %d, Button %v\n", 
		elevatorID, hallCall.Floor, hallCall.Button)

	// Broadcast the hall order assignment to all elevators
	go network.BroadcastHallAssignment(elevatorID, hallCall)
}
