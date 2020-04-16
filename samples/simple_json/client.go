package main

import (
	"fmt"
	"github.com/universe-10th/chasqui"
	"github.com/universe-10th/chasqui/marshalers/json"
	. "github.com/universe-10th/chasqui/types"
	"net"
	"time"
)


type SampleClientFunnel struct {
	clientName string
	closer func()
}


func (funnel SampleClientFunnel) Started(attendant *chasqui.Attendant) {
	fmt.Printf("Local(%s) starting\n", funnel.clientName)
	// noinspection GoUnhandledErrorResult
	attendant.Send("NAME", Args{funnel.clientName}, nil)
}


func (funnel SampleClientFunnel) MessageArrived(attendant *chasqui.Attendant, message Message) {
	fmt.Printf("Local(%s) received: %v\n", funnel.clientName, message)
}


func (SampleClientFunnel) MessageThrottled(*chasqui.Attendant, Message, time.Time, time.Duration) {}


func (funnel SampleClientFunnel) Stopped(attendant *chasqui.Attendant, stopType chasqui.AttendantStopType, err error) {
	fmt.Printf("Local(%s) stopped: %d, %s\n", funnel.clientName, stopType, err)
	funnel.closer()
}


func MakeClient(host, clientName string, onExtraClose func()) (*chasqui.Attendant, error) {
	if addr, err := net.ResolveTCPAddr("tcp", host); err != nil {
		return nil, err
	} else if conn, err := net.DialTCP("tcp", nil, addr); err != nil {
		return nil, err
	} else {
		client := chasqui.NewBasicClient(conn, &json.JSONMessageMarshaler{}, 0, 16)
		chasqui.ClientFunnel(client, SampleClientFunnel{clientName, onExtraClose})
		return client, nil
	}
}
