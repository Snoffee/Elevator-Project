package main

import (
	"Main_project2/elevio"
	"Main_project2/single_elevator"
	"Main_project2/elevator_config"
	"Main_project2/network"
	"Main_project2/network/peers"
	"Main_project2/master_election"
	"Main_project2/peer_monitor"
	"Main_project2/order_assignment"
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
	elevator.InitElevator()

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

	// **Start Peer Monitoring, Master Election, and Order Assignment**
	go peer_monitor.RunMonitorPeers(peerUpdates, lostPeerChan)
	go master_election.RunMasterElection(elevatorStateChan, masterChan)
	go network.RunNetwork(elevatorStateChan)
	go order_assignment.RunOrderAssignment(elevatorStateChan, masterChan, lostPeerChan, orderAssignmentChan)

	// Start polling hardware inputs
	go elevio.PollButtons(drv_buttons)
	go elevio.PollFloorSensor(drv_floors)
	go elevio.PollObstructionSwitch(drv_obstr)
}
