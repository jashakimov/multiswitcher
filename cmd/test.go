package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	packet, err := net.ListenPacket("ip4:igmp", "0.0.0.0")
	if err != nil {
		return
	}
	if err != nil {
		fmt.Println("Failed to open raw socket:", err)
		panic(err)
	}
	ip := "224.0.0.1"
	kek := net.ParseIP(ip)
	for {
		packet.WriteTo(newIgmpMsg(0x16, kek), &net.IPAddr{IP: kek})
		time.Sleep(2 * time.Second)
	}

}

func newIgmpMsg(msgType byte, multicastAddr net.IP) []byte {
	var packet bytes.Buffer
	packet.WriteByte(msgType)
	packet.WriteByte(0x00)
	packet.Write([]byte{0x00, 0x00})
	packet.Write(multicastAddr.To4())

	checksum := calculateChecksum(packet.Bytes())
	binary.BigEndian.PutUint16(packet.Bytes()[2:], checksum)

	return packet.Bytes()
}

func calculateChecksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 < len(data) {
			sum += uint32(data[i])<<8 | uint32(data[i+1])
		} else {
			sum += uint32(data[i]) << 8
		}
	}

	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return uint16(^sum)
}
