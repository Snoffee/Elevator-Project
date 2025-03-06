# Elevator-Project
TTK4145 Sanntidsprogramering

![alt text](Overview.png)

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
| `main.go`            | Initializes the system, sets up channels, and starts goroutines. |
| `single_elevator.go` | Handles individual elevator logic (cab calls, movement, state transitions). |
| `order_assignment.go` | Assigns hall calls based on elevator states and reassigns lost orders. |
| `master_election.go` | Elects a master elevator and ensures consistent master updates. |
| `peer_monitor.go`    | Monitors connected elevators and detects failures. |
| `network.go`         | Handles peer communication and broadcasts elevator states. |
| `config.go`          | Defines shared configurations and constants. |

---

## **Communication Overview**
The system is built around **Go channels**, which handle all inter-module communication.

- **Hall calls** from `single_elevator` → Sent to `order_assignment`
- **Assigned hall calls** → Sent back to `single_elevator` for execution
- **Elevator states** → Shared via `network.go`
- **Master election updates** → Sent to `master_election` to ensure proper leadership

---

## **Setup and Running the Project**

Set up environment variables:

- **Windows PowerShell**
$env:ELEVATOR_PORT="15657"
$env:ELEVATOR_ID="elevator_1"

- **Linux (or macOS)**
export ELEVATOR_PORT="15657"
export ELEVATOR_ID="elevator_1"

