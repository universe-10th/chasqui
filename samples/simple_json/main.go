package main

import (
	"bufio"
	"fmt"
	"github.com/universe-10th/chasqui"
	. "github.com/universe-10th/chasqui/types"
	"os"
	"strings"
)


func main() {
	if err := server.Run("0.0.0.0:3000"); err != nil {
		fmt.Printf("An error was raised while trying to start the server at address 0.0.0.0:3000: %s\n", err)
		return
	}
	chasqui.ServerFunnel(server, SampleServerFunnel{})
	defer func() {
		if err := server.Stop(); err != nil {
			fmt.Printf("An error was raised while trying to stop the server: %s", err)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	clients := make(map[string]*chasqui.Attendant)

	for {
		fmt.Print("Enter a command: ")
		if line, err := reader.ReadString('\n'); err != nil {
			fmt.Printf("Stdin: error reading line")
		} else if line == "bye" {
			break
		} else if parts := strings.SplitN(line, " ", 2); len(parts) == 2 {
			parts[1] = strings.Trim(parts[1], "\n ")
			switch parts[0] {
			case "start":
				if _, ok := clients[parts[1]]; ok {
					fmt.Printf("Name in use: %s\n", parts[1])
				} else if attendant, err := MakeClient("127.0.0.1:3000", parts[1], func() { delete(clients, parts[1]) }); err != nil {
					fmt.Printf("Failed to make client %s: %s\n", parts[1], err)
				} else {
					clients[parts[1]] = attendant
					if err := attendant.Start(); err != nil {
						fmt.Printf("Client %s failed to start: %s\n", parts[1], err)
					}
				}
			case "shout":
				if subParts := strings.SplitN(parts[1], " ", 2); len(subParts) != 2 {
					fmt.Printf("Shout has the incorrect number of arguments: %s\n", parts[1])
				} else if attendant, ok := clients[subParts[0]]; ok {
					if err := attendant.Send("SHOUT", Args{subParts[1]}, nil); err != nil {
						fmt.Printf("Client %s failed to shout: %s\n", subParts[1], err)
					}
				} else {
					fmt.Printf("Invalid or unknown name: %s\n", subParts[0])
				}
			case "stop":
				if attendant, ok := clients[parts[1]]; ok {
					// noinspection GoUnhandledErrorResult
					attendant.Stop()
					delete(clients, parts[1])
				} else {
					fmt.Printf("Invalid or unknown name: %s\n", parts[1])
				}
			}
		} else {
			fmt.Printf("Command not understood: %s. Retrying...\n", strings.Trim(line, "\n "))
		}
	}
}
