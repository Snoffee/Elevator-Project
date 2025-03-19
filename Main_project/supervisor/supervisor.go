package main

import (
	"Main_project/network/peers"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
    elevatorID   = os.Getenv("ELEVATOR_ID")   
    elevatorPort = os.Getenv("ELEVATOR_PORT") 
)

func main() {
	if elevatorID == "" || elevatorPort == "" {
		log.Fatal("ELEVATOR_ID or ELEVATOR_PORT is not set! Exiting...")
	}

	log.Printf("Supervisor started for Elevator %s on port %s. Monitoring peer network...", elevatorID, elevatorPort)

	go monitorElevator()
	select {} 
}

// Monitor a Single Elevator via Peer Network
func monitorElevator() {
	peerUpdateChan := make(chan peers.PeerUpdate)

	go peers.Receiver(30001, peerUpdateChan)

	for {
		update := <-peerUpdateChan

		for _, lostElevator := range update.Lost {
			if lostElevator == elevatorID {
				log.Printf("Elevator %s disconnected! Restarting...", lostElevator)
				go restartElevator(lostElevator)
			}
		}
		time.Sleep(1 * time.Second) 
	}
}

func restartElevator(elevatorID string) {
	log.Printf("Restarting Elevator: %s on port %s...", elevatorID, elevatorPort)
	var cmd *exec.Cmd

    if runtime.GOOS == "windows" {
        psCommand := fmt.Sprintf(`Start-Process powershell -WindowStyle Normal -ArgumentList '-Command', 'cd ..; $env:ELEVATOR_ID=\"%s\"; $env:ELEVATOR_PORT=\"%s\"; go run main.go'`, elevatorID, elevatorPort)
        cmd = exec.Command("powershell", "-Command", psCommand)
    } else if runtime.GOOS == "linux" {
        bashCommand := fmt.Sprintf(`cd .. && ELEVATOR_ID="%s" ELEVATOR_PORT="%s" go run main.go`, elevatorID, elevatorPort)
        cmd = exec.Command("bash", "-c", bashCommand)
	}
	cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

	cmd.Start()

	// Add a longer delay after restarting
    time.Sleep(30 * time.Second) 
}