# **Elevator-Project**
TTK4145 Sanntidsprogramering

## **Overview**
This project implements a **multi-elevator system** that can:
- **Process cab and hall calls** efficiently.
- **Assign orders dynamically** using an election-based master system.
- **Detect network failures** and **reassign lost orders** automatically.

The system is built using **Go** and follows a **modular architecture** with clear communication between modules using **Go channels**.

---

## **Project Structure**
| **Module**           | **Description** |
|----------------------|----------------|
| `main`            | Initializes the system, sets up channels, and starts goroutines. |
| `singleElevator` | Handles individual elevator logic (cab calls, movement, state transitions). |
| `orderAssignment` | Assigns hall calls based on elevator states and reassigns lost orders. |
| `masterElection` | Elects a master elevator and ensures consistent master updates. |
| `peerMonitor`    | Monitors connected elevators and detects failures. |
| `network`         | Handles peer communication and broadcasts elevator states. |
| `config`          | Defines shared configurations and constants. |
| `elevio`          | . |
| `communication`   | Handles message sending, elevator status updates and generally manages network functionality. |
| `supervisor`   | Restarts the elevator when it enters a failure state. |


---

## **Architecture**

- **Master-Slave Model:**
The system operates in a master-slave configuration. One elevator is elected as the master, which is responsible for handling hall call assignments, order distribution and reassigning of lost hall calls. In addition, it sends a backup of a resurrected slaves previous cab calls. Other elevators act as slaves, executing assigned hall orders.

- **Master Election:**
If the current master goes offline, the elevator with the lowest ID of the remaining is elected as the new master. A reelection is held every time a new peer enters the network.

- **Communication Protocol:**
All elevators communicate using UDP broadcasting, ensuring that network messages such as peer updates, master elections, and order assignments are efficiently shared.

- **Acknowledgement System:**
All message types are confirmed by the recipient sending an acknowledgement message to the transmitter. The transmitter keeps resending messages untill an acknowledgement is received or it times out.

- **Supervisor:**
Each elevator has its own supervisor that keeps tabs on the executable. It detects when the executable is down and automatically restarts it. Used to handle failure states, like loss of motor power and obstruction problems.

---

## **Communication Overview**
The system is built around **Go channels**, which handle all inter-module communication.

- **Hall calls** from `single_elevator` → Sent to `order_assignment`via `hallCallChan`
- **Assigned hall calls** → Sent back to `single_elevator` for execution: 
	- If this elevator is chosen: via `assignedHallCallChan`
	- If another elevator is chosen: via `network.go`
- **Elevator states** → Broadcasted via `network.go` through `elevatorStateChan` to:
	- **Elect master** in `master_election`
	- **Select an elevator** in `order_assignment` for hall calls
- **Master election updates** → Sent to `order_assignment` via `masterChan` to ensure only the master assigns orders.
- **Lost peer detection** -> Sent to `order_assignment` via `lostPeerChan` to reassign lost orders

---

## **Communication Overview**
![486748424_9977059018973185_4486015035974998800_n](https://github.com/user-attachments/assets/f8352c70-77c4-40a6-b0a4-2dc53cf4ef64)

---

## **Setup and Running the Project**

Set up environment variables:

- **Windows PowerShell**
	- $env:ELEVATOR_PORT="15657"
	- $env:ELEVATOR_ID="elevator_1"

- **Linux (or macOS)**
	- export ELEVATOR_PORT="15657"
	- export ELEVATOR_ID="elevator_1"

To start the elevator system:
- go run main.go
