package main

import (
	"fmt"
	"github.com/universe-10th/chasqui"
	"github.com/universe-10th/chasqui/marshalers/json"
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


func MakeClient(host, clientName string, onExtraClose func()) (*chasqui.Attendant, error) {
	if addr, err := net.ResolveTCPAddr("tcp", host); err != nil {
		return nil, err
	} else if conn, err := net.DialTCP("tcp", nil, addr); err != nil {
		return nil, err
	} else {
		conveyor := make(chan chasqui.Conveyed)
		quitChannel := make(chan bool)

		onClientStart := func(attendant *chasqui.Attendant) {
			// noinspection GoUnhandledErrorResult
			attendant.Send("NAME", Args{clientName}, nil)
			go func() {
				Loop: for {
					select {
					case msg := <-conveyor:
						fmt.Printf(
							"Local(%s) <- %s, %v, %v\n", clientName, msg.Message.Command(),
							msg.Message.Args(), msg.Message.KWArgs(),
						)
					case <-quitChannel:
						break Loop
					}
				}
			}()
		}
		onClientStop := func(attendant *chasqui.Attendant, stopType chasqui.AttendantStopType, err error) {
			fmt.Printf("Local(%s) stopped: %d, %s\n", clientName, stopType, err)
			onExtraClose()
		}
		onClientThrottle := func(attendant *chasqui.Attendant, message Message, instant time.Time, lapse time.Duration) {
			// This function does nothing since throttle is 0.
		}

		return chasqui.NewAttendant(
			conn, &json.JSONMessageMarshaler{}, conveyor, 0, onClientStart, onClientStop, onClientThrottle,
		), nil
	}
}
