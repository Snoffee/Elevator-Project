package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
	"fmt"
)

// Heartbeat struct
type Heartbeat struct {
	ID        string
	Timestamp time.Time
}

var (
	elevatorStatus   = make(map[string]time.Time) // Last received heartbeat
	restartAttempts  = make(map[string]int)      // Track restart attempts
	mu               sync.Mutex
	maxRestarts      = 3                          // Maximum restarts per elevator before cooldown
	cooldownDuration = 30 * time.Second           // Cooldown time
)

// **Listen for elevator heartbeats**
func listenForElevators() {
	udpAddr, _ := net.ResolveUDPAddr("udp", ":30010") // Dedicated port for heartbeats

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP port: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		var receivedData Heartbeat
		decoder := gob.NewDecoder(bytes.NewReader(buffer[:n]))
		err = decoder.Decode(&receivedData)
		if err != nil {
			log.Printf("Failed to decode heartbeat: %v", err)
			continue
		}

		mu.Lock()
		elevatorStatus[receivedData.ID] = time.Now()
		restartAttempts[receivedData.ID] = 0 
		mu.Unlock()
	}
}

// **Monitor elevators and restart if offline**
func monitorElevators() {
	for {
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		for id, lastSeen := range elevatorStatus {
			if time.Since(lastSeen) > 5*time.Second {
				log.Printf("🚨 Elevator %s is unresponsive! Restarting...", id)

				if restartAttempts[id] >= maxRestarts {
					log.Printf("Elevator %s exceeded max restarts. Cooling down...", id)
					delete(elevatorStatus, id)
					time.AfterFunc(cooldownDuration, func() {
						mu.Lock()
						restartAttempts[id] = 0
						mu.Unlock()
					})
					continue
				}

				restartAttempts[id]++
				go restartElevator(id)
				// Give the elevator time to restart
				time.Sleep(15 * time.Second)
			}
		}
		mu.Unlock()
	}
}

// **Restart an elevator**
func restartElevator(elevatorID string) {
	log.Printf("🔄 Restarting Elevator: %s (Attempt %d/%d)...", elevatorID, restartAttempts[elevatorID], maxRestarts)

	elevatorPath := `"C:\Users\synno\OneDrive\Dokumenter\Semester 6\Elevator_Project\Elevator-Project\Main_project"`

	elevatorPorts := map[string]string{
		"elevator_1": "15657",
		"elevator_2": "15658",
		"elevator_3": "15659",
	}

	elevatorPort, exists := elevatorPorts[elevatorID]
	if !exists {
		log.Printf("Unknown elevator ID: %s. Cannot restart.", elevatorID)
		return
	}

	// **Corrected PowerShell Command**
	psCommand := fmt.Sprintf(`Start-Process powershell -WindowStyle Normal -ArgumentList '-NoExit', '-Command', 'Set-Location -LiteralPath \"%s\"; $env:ELEVATOR_ID=\"%s\"; $env:ELEVATOR_PORT=\"%s\"; go run main.go'`,
		elevatorPath, elevatorID, elevatorPort)

	cmd := exec.Command("powershell", "-Command", psCommand)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		log.Printf("Failed to restart Elevator %s: %v", elevatorID, err)
		return
	}

	log.Printf("Successfully restarted Elevator %s on port %s in a **new PowerShell window**", elevatorID, elevatorPort)
}

// **Main function to start the supervisor**
func main() {
	log.Println("Supervisor started. Listening for heartbeats...")
	go listenForElevators()
	go monitorElevators()
	select {} 
}
