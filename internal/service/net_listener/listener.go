package net_listener

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/jashakimov/multiswitcher/internal/utils"
	"log"
)

type Listener interface {
	Receive(ip string, info Info)
	Stop(ip string)
}

type service struct {
	ips          *utils.SyncMap[string, Info]
	packetSource *gopacket.PacketSource
}

func NewService(iname string) Listener {
	handle, err := pcap.OpenLive(iname, 65536, true, pcap.BlockForever)
	if err != nil {
		log.Fatal(err)
	}

	err = handle.SetBPFFilter("ip")
	if err != nil {
		panic(err)
	}

	s := service{
		ips:          utils.NewSyncMap[string, Info](),
		packetSource: gopacket.NewPacketSource(handle, handle.LinkType()),
	}

	go s.listen()

	return &s
}

func (s *service) Receive(ip string, info Info) {
	fmt.Println("Добавляем прослушку ", ip)
	s.ips.Set(ip, info)
}

func (s *service) Stop(ip string) {
	fmt.Println("Останавливаем прослушку ", ip)
	s.ips.Del(ip)
}

func (s *service) listen() {
	for packet := range s.packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			pack, ok := ipLayer.(*layers.IPv4)
			if !ok {
				continue
			}
			dspIp := pack.DstIP.String()
			if ch, ok := s.ips.Get(dspIp); ok {
				ch.ReceiveChan <- ch.Id
			}
		}
	}
}
