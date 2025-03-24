package orderAssignment

import (
	"mainProject/config"
	"mainProject/elevio"
	"mainProject/masterElection"
	"mainProject/network"
	"fmt"
)

// **Run Order Assignment as a Goroutine**
func RunOrderAssignment(elevatorStatusesChan chan map[string]network.ElevatorStatus, masterChan chan string, lostPeerChan chan string, newPeerChan chan string, hallCallChan chan elevio.ButtonEvent, assignedHallCallChan chan elevio.ButtonEvent, orderStatusChan chan network.OrderStatusMessage, txAckChan chan network.AckMessage) {

	go func() {
		var latestMasterID string
		var latestElevatorStatuses map[string]network.ElevatorStatus

		for {
			select {
			case updatedStatuses := <-elevatorStatusesChan:
				latestElevatorStatuses = updatedStatuses 

			case newMaster := <-masterChan:
				latestMasterID = newMaster 
				fmt.Printf("Updated Master ID: %s\n\n", latestMasterID)

			case lostElevator := <-lostPeerChan:
				if lostElevator == latestMasterID {
					fmt.Printf("Master elevator %s disconnected! Reassigning hall orders...\n\n", lostElevator)
					masterElection.ElectMaster(latestElevatorStatuses, masterChan)
					newMasterID := <-masterChan
					latestMasterID = newMasterID
				}
				if latestMasterID == config.LocalID && latestElevatorStatuses != nil {
					fmt.Printf("Lost elevator detected: %s. Reassigning hall orders...\n\n", lostElevator)
					ReassignLostHallOrders(lostElevator, latestElevatorStatuses, assignedHallCallChan)
				}
			case newElevator := <-newPeerChan:
				if latestMasterID == config.LocalID && latestElevatorStatuses != nil {
					fmt.Printf("New elevator detected: %s. Restoring cab calls...\n\n", newElevator)
					ReassignCabCalls(newElevator)
				}
			case hallCall := <-hallCallChan: 
				if latestMasterID == config.LocalID {
					bestElevator := AssignHallOrder(hallCall.Floor, hallCall.Button, latestElevatorStatuses, "") // Passing "" on excludeElevator when normally calling AssignHallOrder		
					
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
					ackMsg := network.AckMessage{TargetID: status.SenderID, SeqNum: status.SeqNum}
					fmt.Printf("Sending ack for orderStatus to sender: %s | SeqNum: %d\n\n", ackMsg.TargetID, ackMsg.SeqNum)
					txAckChan <- ackMsg 
					
					if status.Status == network.Unfinished {
						fmt.Printf("Received unfinished order status from elevator %s\n", status.SenderID)
						network.SendLightOrder(status.ButtonEvent, network.On)
						elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, true)
						fmt.Printf("Turned on order hall light for all elevators\n\n")
					} else {
						network.SendLightOrder(status.ButtonEvent, network.Off)
						fmt.Printf("Received finished order status from elevator %s\n", status.SenderID)
						elevio.SetButtonLamp(status.ButtonEvent.Button, status.ButtonEvent.Floor, false)
						fmt.Printf("Turned off order hall light for all elevators\n\n")
					}
				}
		}
	}
	}()
}

// **Reassign hall orders if an elevator disconnects**
func ReassignLostHallOrders(lostElevator string, elevatorStatuses map[string]network.ElevatorStatus, assignedHallCallChan chan elevio.ButtonEvent) {
	state, exists := elevatorStatuses[lostElevator];
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
			bestElevator := AssignHallOrder(floor, elevio.ButtonType(button), elevatorStatuses, lostElevator) 
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
func AssignHallOrder(floor int, button elevio.ButtonType, elevatorStatuses map[string]network.ElevatorStatus, excludeElevator string) string {
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

// **Helper function to calculate absolute distance**
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

