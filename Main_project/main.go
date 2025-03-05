package main

import (
	"Main_project/elevio"
	"Main_project/single_elevator"
	"Main_project/config"
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

	// Channels for hardware events
	drv_buttons := make(chan elevio.ButtonEvent)
	drv_floors := make(chan int)
	drv_obstr := make(chan bool)

	peerUpdates := make(chan peers.PeerUpdate)
	elevatorStateChan := make(chan map[string]network.ElevatorStatus) // Elevator state updates
	masterChan := make(chan string, 1)             // Master election results
	orderAssignmentChan := make(chan int)          // Assigned orders to single elevator
	lostPeerChan := make(chan string)				// Lost peers
	heartbeatChan := make(chan string, 10) 			// Heartbeat channel

	// **Start Peer Monitoring, Master Election, and Order Assignment**
	go peer_monitor.RunMonitorPeers(peerUpdates, lostPeerChan)
	go master_election.RunMasterElection(elevatorStateChan, masterChan, heartbeatChan)
	go network.RunNetwork(elevatorStateChan)
	go order_assignment.RunOrderAssignment(elevatorStateChan, masterChan, lostPeerChan, orderAssignmentChan)
	go master_election.ReceiveMasterUpdates(masterChan)

	// Start polling hardware inputs
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
}
