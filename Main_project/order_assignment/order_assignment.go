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
				fmt.Printf("Updated Master ID: %s\n\n", latestMasterID)

			case lostElevator := <-lostPeerChan:
				fmt.Printf("Lost elevator detected: %s. Reassigning orders...\n\n", lostElevator)
				if latestMasterID == config.LocalID && latestElevatorStates != nil {
					ReassignLostOrders(lostElevator, latestElevatorStates, assignedHallCallChan)
				}
			case hallCall := <-hallCallChan: // Receives a hall call from single_elevator
				if latestMasterID == config.LocalID {
					// Check if the hall call is already assigned
					bestElevator := AssignHallOrder(hallCall.Floor, hallCall.Button, latestElevatorStates)
					if !latestElevatorStates[bestElevator].Queue[hallCall.Floor][hallCall.Button] {
						
						if bestElevator == config.LocalID {
							// If this elevator was chosen, send it back to `single_elevator`
							fmt.Printf("Assigned hall call to this elevator at floor %d\n\n", hallCall.Floor)
							assignedHallCallChan <- hallCall
						} else {
							// If another elevator was chosen, send assignment over network
							network.SendHallAssignment(bestElevator, hallCall.Floor, hallCall.Button)
						}
					} else {
						fmt.Printf("Ignoring duplicate assignment of hall call at floor %d\n\n", hallCall.Floor)
					}
				} else {
					// If the slave gets the hall order, send order on network (Forwarding hall call to master)
					network.SendRawHallCall(hallCall)
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
					fmt.Printf("Order at floor %d successfully reassigned to %s\n\n", floor, bestElevator)
					if bestElevator == config.LocalID {
						// Send re-assigned order to this elevator
						assignedHallCallChan <- elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)}
					} else {
						// Otherwise, notify the chosen elevator over the network
						network.SendHallAssignment(bestElevator, floor, elevio.ButtonType(button))
					}
				} else {
					fmt.Printf("No available elevator for reassignment!\n\n")
				}
			}
		}
	}
}

// **Assign hall order to the closest available elevator**
func AssignHallOrder(floor int, button elevio.ButtonType, elevatorStates map[string]network.ElevatorStatus) string {
	fmt.Println("Available elevators:", elevatorStates)
	fmt.Println()

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
	fmt.Println()

	// Assign the order to the best elevator
	if bestElevator != "" {
		fmt.Printf("Assigning hall call at floor %d to %s\n\n", floor, bestElevator)
		network.SendHallAssignment(bestElevator, floor, button)

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

