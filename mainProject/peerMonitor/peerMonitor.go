package peerMonitor

import (
	"mainProject/config"
	"mainProject/network"
	"mainProject/network/peers"
	"time"
	"fmt"
)

// **Runs MonitorPeers as a Goroutine**
func RunMonitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string, newPeerChan chan string) {
	go monitorPeers(peerUpdateChan, lostPeerChan, newPeerChan)
	go announceSelf()
}

// **Monitor Peers and Notify Master Election & Order Assignment**
func monitorPeers(peerUpdateChan chan peers.PeerUpdate, lostPeerChan chan string, newPeerChan chan string) {
	for update := range peerUpdateChan {
		fmt.Printf("Received peer update: New=%v, Lost=%v\n", update.New, update.Lost)
		network.UpdateElevatorStates(update.New, update.Lost)

		// Notify about lost elevators
		for _, lostPeer := range update.Lost {
			fmt.Printf("Elevator %s disconnected!\n", lostPeer)
			lostPeerChan <- lostPeer
		}
		// Notify about newly joined elevators
		for _, newPeer := range update.New {
			fmt.Printf("Elevator %s has joined!\n", newPeer)
			newPeerChan <- newPeer
		}
	}
}

// **Announce Self to the Network**
func announceSelf() {
	txEnable := make(chan bool, 1)
	txEnable <- true

	go peers.Transmitter(30001, config.LocalID, txEnable) 

	for {
		time.Sleep(1 * time.Second) 
	}
}

