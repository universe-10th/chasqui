package main

import (
	"fmt"
	"github.com/universe-10th/chasqui"
	"github.com/universe-10th/chasqui/marshalers/json"
	. "github.com/universe-10th/chasqui/types"
)


func lifecycle(basicServer *chasqui.BasicServer) {
	Loop: for {
		select {
		case <-basicServer.StartedEvent():
			fmt.Println("The server has started successfully")
		case event := <-basicServer.AcceptFailedEvent():
			fmt.Printf("An error was raised while trying to accept a new incoming connection: %s\n", event)
		case <-basicServer.StoppedEvent():
			fmt.Printf("The server has stopped successfully")
			break Loop
		case event := <-basicServer.AttendantStartedEvent():
			// noinspection GoUnhandledErrorResult
			event.Attendant.Send("Hello", nil, nil)
		case event := <-basicServer.MessageEvent():
			attendant := event.Attendant
			message := event.Message
			name, _ := attendant.Context("name")
			fmt.Printf("Remote(%s) -> A new message arrived: %s\n", name, message.Command())
			switch message.Command() {
			case "NAME":
				args := message.Args()
				if len(args) == 1 {
					attendant.SetContext("name", args[0])
					// noinspection GoUnhandledErrorResult
					attendant.Send("NAME_OK", Args{args[0]}, nil)
				} else {
					// noinspection GoUnhandledErrorResult
					attendant.Send("NAME_MISSING", nil, nil)
				}
			case "SHOUT":
				args := message.Args()
				if name, _ := attendant.Context("name"); name == nil {
					if err := attendant.Send("NAME_MUST", nil, nil); err != nil {
						fmt.Printf("Remote: Failed to respond NAME_MUST: %s\n", err)
					}
				} else if len(args) != 1 {
					if err := attendant.Send("SHOUT_MISSING", nil, nil); err != nil {
						fmt.Printf("Remote: Failed to respond SHOUT_MISSING to %s: %s\n", name, err)
					}
				} else {
					server.Enumerate(func(target *chasqui.Attendant) {
						if err := target.Send("SHOUTED", Args{name, args[0]}, nil); err != nil {
							fmt.Printf("Remote: Failed to broadcast SHOUTED from %s: %s\n", name, err)
						}
					})
				}
			}
		case <-basicServer.ThrottledEvent():
		case <-basicServer.AttendantStoppedEvent():
		}
	}
}


var server = chasqui.NewServer(
	&json.JSONMessageMarshaler{}, 1024, 1, 0,
)


