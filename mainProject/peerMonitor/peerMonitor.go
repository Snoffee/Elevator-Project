package peerMonitor

import (
	"fmt"
	"mainProject/config"
	"mainProject/communication"
	"mainProject/network/peers"
	"mainProject/singleElevator"
)

// Runs MonitorPeers as a Goroutine
func RunMonitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string, newPeerChan chan string, localStatusUpdateChan chan config.Elevator) {
	go monitorPeers(peerUpdateChan, lostPeerChan, newPeerChan, localStatusUpdateChan)
	
	txEnable := make(chan bool, 1)
	txEnable <- true

	go peers.Transmitter(30001, config.LocalID, txEnable) 
}

// Monitor Peers and Notify Master Election & Order Assignment
func monitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string, newPeerChan chan string, localStatusUpdateChan chan config.Elevator) {
	for update := range peerUpdateChan {
		fmt.Printf("Received peer update: New=%v, Lost=%v\n", update.New, update.Lost)
		communication.UpdateElevatorStates(update.New, update.Lost)

		for _, lostPeer := range update.Lost {
			lostPeerChan <- lostPeer
		}
		for _, newPeer := range update.New {
			newPeerChan <- newPeer
			localStatusUpdateChan <- singleElevator.GetElevatorState()
		}
	}
}


