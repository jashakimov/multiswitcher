package igmp

import (
	"fmt"
	"net"
)

type Connection interface {
	Send(msg []byte, ip net.IP)
	Close()
}

type connection struct {
	conn net.PacketConn
}

func NewConnection(ip string) (Connection, error) {
	c := &connection{}
	var err error

	c.conn, err = net.ListenPacket("ip4:igmp", ip)
	if err != nil {
		fmt.Println("Failed to open raw socket:", err)
		return nil, err
	}
	return c, err
}

func (c *connection) Send(msg []byte, ip net.IP) {
	c.conn.WriteTo(msg, &net.IPAddr{IP: ip})
}

func (c *connection) Close() {
	c.conn.Close()
}
