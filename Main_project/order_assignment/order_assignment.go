// In: 	
//		elevatorStateChan (from network.go) → Assigns orders dynamically.
//		masterChan (from master_election.go) → Ensures only the master assigns orders.
//		lostPeerChan (from peer_monitor.go) → Reassigns orders if an elevator disconnects.	

// Out:
//		orderAssignmentChan (used by single_elevator.go) → Assigns orders to elevators.

package order_assignment

import (
	"Main_project2/elevator_config"
	"Main_project2/network"
	"fmt"
)

// **Run Order Assignment as a Goroutine**
func RunOrderAssignment(elevatorStateChan chan map[string]network.ElevatorStatus, masterChan chan string, lostPeerChan chan string, orderAssignmentChan chan int) {
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
					ReassignLostOrders(lostElevator, latestElevatorStates)
				}
			}
		}
	}()
}

// **Reassign orders if an elevator disconnects**
func ReassignLostOrders(lostElevator string, elevatorStates map[string]network.ElevatorStatus) {
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

				bestElevator := AssignHallOrder(floor, button, elevatorStates) // Reassign order

				if bestElevator != "" {
					fmt.Printf("Order at floor %d successfully reassigned to %s\n", floor, bestElevator)
				} else {
					fmt.Printf("No available elevator for reassignment!\n")
				}
			}
		}
	}
}

// **Assign hall order to the closest available elevator**
func AssignHallOrder(floor int, button int, elevatorStates map[string]network.ElevatorStatus) string {
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
		SendHallAssignment(bestElevator, floor, button)
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
func SendHallAssignment(elevatorID string, floor int, button int) {
	fmt.Printf("Sending hall order to elevator %s: Floor %d, Button %v\n", elevatorID, floor, button)
	// Here, implement network communication to inform `single_elevator` about the order
}
