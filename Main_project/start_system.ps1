# This script starts the simulator, elevator, and supervisor for a given elevator ID and port number.
# Start the system by running 
# Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
# .\start_system.ps1 -ElevatorID "elevator_2" -ElevatorPort "15658"

param (
    [string]$ElevatorID = "elevator_1",
    [string]$ElevatorPort = "15657"
)

Write-Host "Starting Simulator for $ElevatorID on port $ElevatorPort..."
Start-Process -FilePath "powershell" -ArgumentList "-NoExit", "-Command", "`$env:SERVER_PORT='$ElevatorPort'; cd ../Simulator; .\SimElevatorServer.exe" -WindowStyle Normal

Start-Sleep -Seconds 5

Write-Host "Starting Elevator $ElevatorID on port $ElevatorPort..."
Start-Process -FilePath "powershell" -ArgumentList "-NoExit", "-Command", "`$env:ELEVATOR_ID='$ElevatorID'; `$env:ELEVATOR_PORT='$ElevatorPort'; go run main.go" -WindowStyle Normal

Start-Sleep -Seconds 5

Write-Host "Starting Supervisor for $ElevatorID on port $ElevatorPort..."
Start-Process -FilePath "powershell" -ArgumentList "-NoExit", "-Command", "`$env:ELEVATOR_ID='$ElevatorID'; `$env:ELEVATOR_PORT='$ElevatorPort'; cd ./supervisor; go run supervisor.go" -WindowStyle Normal

Write-Host "System started! Simulator, Elevator, and Supervisor are running."



