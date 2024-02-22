package main

import (
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	_ "github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	var fileConfig string
	flag.StringVar(&fileConfig, "config", "", "path to config file")
	flag.Parse()
	if fileConfig == "" {
		fmt.Println("No file directory")
	}
	/////////////////////////////////////////////////////////////
	cfg := NewConfig(fileConfig)

	printConfig(cfg)

	lo, err := netlink.LinkByName(cfg.Interface)
	if err != nil {
		panic(err)
	}

	log.Printf("Установка мультикаста на интерфейс: %s\n", cfg.Interface)
	// установка мултикаста
	if err := LinkSetMulticast(lo); err != nil {
		panic(err)
	}

	log.Printf("Установка промискуитетного режима на интерфейс: %s\n", cfg.Interface)
	// установка промискуитетного режима
	if err := netlink.SetPromiscOn(lo); err != nil {
		panic(err)
	}

	log.Printf("Установка qdisc на интерфейс: %s\n", cfg.Interface)
	// установка дисциплины, для последующей установки фильтров
	if err := SetIngressQDisc(lo); err != nil {
		fmt.Println(err)
	}

	// установка маршрутизации роутеров
	if err := Route(lo, cfg.Filters); err != nil {
		panic(err)
	}

	// Открываем сетевой интерфейс для захвата пакетов
	handle, err := pcap.OpenLive(cfg.Interface, 1024, true, time.Second*1)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	log.Println("Установка мастер фильтров")
	for _, filter := range cfg.Filters {
		DelFilter(lo.Attrs().Name, filter.Master.Priority, filter.Master.IP, filter.Route)
		DelFilter(lo.Attrs().Name, filter.Slave.Priority, filter.Slave.IP, filter.Route)
		// установка мастер фильтров по умолчанию
		AddFilter(lo.Attrs().Name, filter.Master.Priority, filter.Master.IP, filter.Route)

		go func(fil Filter) {
			var tries int
			var isSlaveActual bool

			for packet := range packetSource.Packets() {
				ipLayer := packet.Layer(layers.LayerTypeIPv4)
				if ipLayer != nil {
					if ip, ok := ipLayer.(*layers.IPv4); ok {
						isMaster := strings.Compare(ip.DstIP.String(), fil.Master.IP) == 0
						isSlave := strings.Compare(ip.DstIP.String(), fil.Slave.IP) == 0
						// если мастер перестал присылаться, а слейв есть
						if !isMaster && isSlave {
							tries++
							if fil.AutoSwitch && tries > fil.SwitchTries && !isSlaveActual {
								DelFilter(lo.Attrs().Name, fil.Master.Priority, fil.Master.IP, fil.Route)
								AddFilter(lo.Attrs().Name, fil.Slave.Priority, fil.Slave.IP, fil.Route)
								isSlaveActual = true
							}
						} else {
							// обнуляем счетчик попыток
							tries = 0
						}
						fmt.Println("destIP", ip.DstIP.String())
					}
				}
			}
		}(filter)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func printConfig(cfg *Config) {
	fmt.Println("Total pairs:", len(cfg.Filters), "for Interface:", cfg.Interface)
	for i, filter := range cfg.Filters {
		fmt.Printf(" %d) masterIP: '%s', slaveIP: '%s', changeIP: '%s', tries before switch: '%d'\n",
			i+1,
			filter.Master.IP,
			filter.Slave.IP,
			filter.Route,
			filter.SwitchTries,
		)
	}
}

func listenInterface(cfg *Config, lo netlink.Link) {

}

//func listenInterface(interfaceName string) {
//	// Открываем сетевой интерфейс для захвата пакетов
//	handle, err := pcap.OpenLive(interfaceName, 1024, true, time.Second*3)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer handle.Close()
//
//	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
//
//	fmt.Println("Отслеживание трафика. Для выхода нажмите Ctrl+C.")
//
//	// Запускаем бесконечный цикл для анализа каждого пакета
//	for packet := range packetSource.Packets() {
//		ipLayer := packet.Layer(layers.LayerTypeIPv4)
//		if ipLayer != nil {
//			ip, _ := ipLayer.(*layers.IPv4)
//			fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
//			fmt.Println("Protocol: ", ip.Protocol)
//			fmt.Println("Bytes: ", ip.Length)
//		}
//	}
//}

func AddFilter(interfaceName string, priority int, ip, route string) {
	cmd := exec.Command(
		"tc", "filter", "add", "dev", interfaceName, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(priority), "u32",
		"match", "ip", "dst", ip,
		"action", "nat", "ingress", ip, route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка добавление мастер-фильтра")
	}
}
func DelFilter(interfaceName string, priority int, ip, route string) {
	cmd := exec.Command(
		"tc", "filter", "delete", "dev", interfaceName, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(priority), "u32",
		"match", "ip", "dst", ip,
		"action", "nat", "ingress", ip, route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка удаления master-фильтра: filter does not exist")
	}
}

func SetIngressQDisc(lnk netlink.Link) interface{} {
	qDisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			Parent:    netlink.HANDLE_INGRESS,
			LinkIndex: lnk.Attrs().Index,
			Handle:    netlink.HANDLE_NONE,
		},
	}

	return netlink.QdiscAdd(qDisc)
}

func Route(lnk netlink.Link, filters []Filter) error {
	for _, filter := range filters {
		ipParsed := net.ParseIP(filter.Route)

		route := &netlink.Route{
			Dst: &net.IPNet{
				IP:   ipParsed,
				Mask: net.CIDRMask(32, 32),
			},
			LinkIndex: lnk.Attrs().Index,
		}

		if err := netlink.RouteDel(route); err != nil {
			log.Println("Попытка установки маршрутизации")
		}

		if err := netlink.RouteAdd(route); err != nil {
			return err
		}
		log.Printf("Установка маршрутизации %s на интерфейс: %s\n", filter.Route, lnk.Attrs().Name)
	}

	return nil
}

func LinkSetMulticast(lnk netlink.Link) error {
	base := lnk.Attrs()
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK)

	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Change = unix.IFF_MULTICAST
	msg.Flags = unix.IFF_MULTICAST

	msg.Index = int32(base.Index)
	req.AddData(msg)

	_, err := req.Execute(unix.NETLINK_ROUTE, 0)

	return err
}
