package lnet

import (
	"fmt"
	"net"
)

type TcpNid struct {
	IPAddress      *net.IP
	driverInstance int
}

func (t *TcpNid) Address() interface{} {
	return t.IPAddress
}

func (t *TcpNid) Driver() string {
	return "tcp"
}

func (t *TcpNid) LNet() string {
	return fmt.Sprintf("%s%d", t.Driver(), t.driverInstance)
}

func newTcpNid(address string, driverInstance int) (*TcpNid, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, fmt.Errorf("%q is not a valid IP address", address)
	}
	return &TcpNid{
		IPAddress:      &ip,
		driverInstance: driverInstance,
	}, nil
}