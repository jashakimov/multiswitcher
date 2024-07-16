package igmp

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"net"
)

type Connection interface {
	Send(msg []byte, ip net.IP)
	Leave(iface string, ip net.IP)
	Join(iface string, ip net.IP)
	Close()
}

type connection struct {
	conn net.PacketConn
	pack *ipv4.PacketConn
}

func NewConnection(ip string) (Connection, error) {
	c := &connection{}
	var err error

	c.conn, err = net.ListenPacket("ip4:igmp", ip)
	if err != nil {
		fmt.Println("Failed to open raw socket:", err)
		return nil, err
	}

	c.pack = ipv4.NewPacketConn(c.conn)

	return c, err
}

func (c *connection) Send(msg []byte, ip net.IP) {
	c.conn.WriteTo(msg, &net.IPAddr{IP: ip})
}

func (c *connection) Close() {
	c.conn.Close()
	c.pack.Close()
}

func (c *connection) Leave(iface string, ip net.IP) {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		fmt.Printf("Ошибка при отписке от группы: %v\n", err)
		return
	}

	if err := c.pack.LeaveGroup(i, &net.UDPAddr{IP: ip}); err != nil {
		fmt.Printf("Ошибка при отписке от группы: %v\n", err)
		return
	}
}

func (c *connection) Join(iface string, ip net.IP) {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		fmt.Printf("Ошибка при подписке на группу: %v\n", err)
		return
	}

	if err := c.pack.JoinGroup(i, &net.UDPAddr{IP: ip}); err != nil {
		fmt.Printf("Ошибка при подписке на группу: %v\n", err)
		return
	}
}
