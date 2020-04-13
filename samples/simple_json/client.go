package main

import (
	"fmt"
	"github.com/universe-10th/chasqui"
	"github.com/universe-10th/chasqui/marshalers/json"
	. "github.com/universe-10th/chasqui/types"
	"net"
)


func MakeClient(host, clientName string, onExtraClose func()) (*chasqui.Attendant, error) {
	if addr, err := net.ResolveTCPAddr("tcp", host); err != nil {
		return nil, err
	} else if conn, err := net.DialTCP("tcp", nil, addr); err != nil {
		return nil, err
	} else {
		client := chasqui.NewClient(conn, &json.JSONMessageMarshaler{}, 0, 16)
		go func(){
			Loop: select {
			case event := <-client.StartedEvent():
				fmt.Printf("Local(%s) starting, %s\n", clientName, err)
				// noinspection GoUnhandledErrorResult
				event.Attendant.Send("NAME", Args{clientName}, nil)
			case event := <-client.StoppedEvent():
				fmt.Printf("Local(%s) stopped: %d, %s\n", clientName, event.StopType, err)
				onExtraClose()
				break Loop
			case event := <-client.MessageEvent():
				fmt.Printf("Local(%s) received: %v", event.Message)
			case <-client.ThrottledEvent():
				// Nothing here.
			}
		}()
		return client, nil
	}
}
