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
	Receive(ip string, receiveChan chan struct{})
	Stop(ip string)
}

type service struct {
	ips          *utils.SyncMap[string, chan struct{}]
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
		ips:          utils.NewSyncMap[string, chan struct{}](),
		packetSource: gopacket.NewPacketSource(handle, handle.LinkType()),
	}

	go s.listen()

	return &s
}

func (s *service) Receive(ip string, receiveChan chan struct{}) {
	fmt.Println("Добавляем прослушку ", ip)
	s.ips.Set(ip, receiveChan)
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
			if ch, ok := s.ips.Get(pack.DstIP.String()); ok {
				fmt.Println("Пришли байты с ", pack.DstIP.String(), " переключаемся обратно на мастер")
				ch <- struct{}{}
				s.Stop(pack.DstIP.String())
			}
		}
	}
}
