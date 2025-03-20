package main

import (
	"mainProject/elevio"
	"mainProject/config"
	"mainProject/singleElevator"
	"mainProject/network"
	"mainProject/network/peers"
	"mainProject/masterElection"
	"mainProject/peerMonitor"
	"mainProject/orderAssignment"
)

func main() {
	singleElevator.InitElevator()
	config.InitConfig()

	peerUpdates := make(chan peers.PeerUpdate)
	elevatorStateChan := make(chan map[string]network.ElevatorStatus) // Elevator state updates
	masterChan := make(chan string, 1)             // Master election results
	lostPeerChan := make(chan string)				// Lost peers
	newPeerChan := make(chan string)				// New peers
	hallCallChan := make(chan elevio.ButtonEvent, 20)  // Send hall calls to order_assignment
	orderStatusChan := make(chan network.OrderStatusMessage, 20) // Send confirmation of hall calls
	assignedHallCallChan := make(chan elevio.ButtonEvent, 20) // Receive assigned hall calls

	// Start single_elevator
	go singleElevator.RunSingleElevator(hallCallChan, assignedHallCallChan, orderStatusChan)

	// Start Peer Monitoring
	go peerMonitor.RunMonitorPeers(peerUpdates, lostPeerChan, newPeerChan)
	
	// Start Master Election 
	go masterElection.RunMasterElection(elevatorStateChan, masterChan)

	// Start Network
	go network.RunNetwork(elevatorStateChan, peerUpdates, orderStatusChan)
	
	// Start Order Assignment
	go orderAssignment.RunOrderAssignment(elevatorStateChan, masterChan, lostPeerChan, newPeerChan, hallCallChan, assignedHallCallChan, orderStatusChan)

	select{}

}
