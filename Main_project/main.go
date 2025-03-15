package main

import (
	"Main_project/elevio"
	"Main_project/config"
	"Main_project/single_elevator"
	"Main_project/network"
	"Main_project/network/peers"
	"Main_project/master_election"
	"Main_project/peer_monitor"
	"Main_project/order_assignment"
	"fmt"
	"os"
)

func main() {
	fmt.Println("Initializing connection to simulator...")

	port := os.Getenv("ELEVATOR_PORT")
	if port == "" {
    	port = "15657" // Default
	}
	elevio.Init("localhost:" + port, config.NumFloors)

	// Initialize elevator state
	single_elevator.InitElevator()
	// Initialize local ID
	config.InitConfig()
	fmt.Printf("This elevator's ID: %s\n", config.LocalID)

	peerUpdates := make(chan peers.PeerUpdate)
	elevatorStateChan := make(chan map[string]network.ElevatorStatus) // Elevator state updates
	masterChan := make(chan string, 1)             // Master election results
	lostPeerChan := make(chan string)				// Lost peers
	newPeerChan := make(chan string)				// New peers
	hallCallChan := make(chan elevio.ButtonEvent, 20)  // Send hall calls to order_assignment
	orderStatusChan := make(chan network.OrderStatusMessage, 20) // Send confirmation of hall calls
	assignedHallCallChan := make(chan elevio.ButtonEvent, 20) // Receive assigned hall calls

	// Start single_elevator
	go single_elevator.RunSingleElevator(hallCallChan, assignedHallCallChan, orderStatusChan)

	// Start Peer Monitoring
	go peer_monitor.RunMonitorPeers(peerUpdates, lostPeerChan, newPeerChan)
	
	// Start Master Election 
	go master_election.RunMasterElection(elevatorStateChan, masterChan)

	// Start Network
	go network.RunNetwork(elevatorStateChan, peerUpdates, orderStatusChan)
	
	// Start Order Assignment
	go order_assignment.RunOrderAssignment(elevatorStateChan, masterChan, lostPeerChan, newPeerChan, hallCallChan, assignedHallCallChan, orderStatusChan)

	select{}

}
