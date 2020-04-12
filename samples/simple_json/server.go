package main

import (
	"fmt"
	"github.com/universe-10th/chasqui"
	"github.com/universe-10th/chasqui/marshalers/json"
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


var quitChannels = make(map[*chasqui.BasicServer]chan bool)


func started(server *chasqui.BasicServer, addr *net.TCPAddr) {
	quitChannel := make(chan bool)
	quitChannels[server] = quitChannel
	go func(){
		Loop: for {
			select {
			case <-quitChannel:
				break Loop
			case conveyed := <-server.Conveyor():
				attendant := conveyed.Attendant
				message := conveyed.Message
				fmt.Printf("Remote(%s) -> A new message arrived: %s\n", attendant.Context("name"), message.Command())
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
					if name := attendant.Context("name"); name == nil {
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
			}
		}
	}()
	fmt.Println("The server has started successfully")
}


func acceptError(server *chasqui.BasicServer, err error) {
	fmt.Printf("An error was raised while trying to accept a new incoming connection: %s\n", err)
}


func stopped(server *chasqui.BasicServer) {
	quitChannels[server] <- true
	delete(quitChannels, server)
	fmt.Printf("The server has stopped successfully")
}


func attendantStarted(attendant *chasqui.Attendant) {
	// noinspection GoUnhandledErrorResult
	attendant.Send("Hello", nil, nil)
}


func attendantStopped(attendant *chasqui.Attendant, stopType chasqui.AttendantStopType, err error) {
	fmt.Printf("This connection is stopping: %d, %s\n", stopType, err)
	// noinspection GoUnhandledErrorResult
	attendant.Send("Good bye", nil, nil)
}


func throttle(attendant *chasqui.Attendant, message Message, instant time.Time, lapse time.Duration) {
	// This function does nothing since throttle is 0.
}


var server = chasqui.NewServer(
	&json.JSONMessageMarshaler{}, 1024, 0,
	started, acceptError, stopped, attendantStarted, attendantStopped, throttle,
)


