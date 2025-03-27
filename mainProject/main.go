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
	config.InitConfig()
	singleElevator.InitElevator()

	peerUpdatesChan 	 := make(chan peers.PeerUpdate)
	elevatorStatusesChan := make(chan map[string]network.ElevatorStatus) 
	masterElectionChan   := make(chan string, 1)            
	lostPeerChan 		 := make(chan string)				
	newPeerChan          := make(chan string)				
	hallCallChan         := make(chan elevio.ButtonEvent, 20)  // Send hall calls to order_assignment
	orderStatusChan      := make(chan network.OrderStatusMessage, 20) // Send confirmation of hall calls
	assignedHallCallChan := make(chan elevio.ButtonEvent, 20) // Receive assigned hall calls
	txAckChan			 := make(chan network.AckMessage, 20)

	// Start single_elevator
	go singleElevator.RunSingleElevator(hallCallChan, assignedHallCallChan, orderStatusChan, txAckChan)

	// Start Peer Monitoring
	go peerMonitor.RunMonitorPeers(peerUpdatesChan, lostPeerChan, newPeerChan)
	
	// Start Master Election 
	go masterElection.RunMasterElection(elevatorStatusesChan, masterElectionChan)

	// Start Network
	go network.RunNetwork(elevatorStatusesChan, peerUpdatesChan, orderStatusChan, txAckChan)
	
	// Start Order Assignment
	go orderAssignment.RunOrderAssignment(elevatorStatusesChan, masterElectionChan, lostPeerChan, newPeerChan, hallCallChan, assignedHallCallChan, orderStatusChan, txAckChan)

	select{}

}
