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
	"os/exec"
	"strconv"
	"time"
)

func main() {
	var fileConfig string
	flag.StringVar(&fileConfig, "config", "", "path to config file")
	flag.Parse()
	if fileConfig == "" {
		fmt.Println("No file directory")
	}

	cfg := NewConfig(fileConfig)

	lo, err := netlink.LinkByName(cfg.Interface)
	if err != nil {
		fmt.Println(err)
	}

	log.Printf("Установка мультикаста на интерфейс: %s\n", cfg.Interface)
	// установка мултикаста
	if err := LinkSetMulticast(lo); err != nil {
		fmt.Println(err)
	}

	log.Printf("Установка промискуитетного режима на интерфейс: %s\n", cfg.Interface)
	// установка промискуитетного режима
	if err := netlink.SetPromiscOn(lo); err != nil {
		fmt.Println(err)
	}
	log.Printf("Установка маршрутизации %s на интерфейс: %s\n", cfg.Filter.Route, cfg.Interface)
	// установка маршрутизации
	if err := Route(lo, cfg.Filter.Route); err != nil {
		fmt.Println(err)
	}

	log.Printf("Установка qdisc на интерфейс: %s\n", cfg.Interface)
	// установка дисциплины, для последующей установки фильтров
	if err := SetIngressQDisc(lo); err != nil {
		fmt.Println(err)
	}

	// установка мастер-фильтра
	AddMasterFilter(lo, cfg)
	go func() {
		if cfg.Switch {
			ticker := time.NewTicker(cfg.StatFrequencySec * time.Second)
			t := true
			for range ticker.C {
				fmt.Print(lo.Attrs().Statistics)

				tumbler(t, lo, cfg)
				t = !t
			}
		}
	}()

	// Открываем сетевой интерфейс для захвата пакетов
	handle, err := pcap.OpenLive(cfg.Interface, 1600, true, time.Second*3)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	fmt.Println("Отслеживание трафика. Для выхода нажмите Ctrl+C.")

	// Запускаем бесконечный цикл для анализа каждого пакета
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
			fmt.Println("Protocol: ", ip.Protocol)
			fmt.Println("Bytes: ", ip.Length)
			fmt.Println("Info: ", ip)
			fmt.Println()
		}
	}

	time.Sleep(time.Hour)
}

func tumbler(b bool, lnk netlink.Link, cfg *Config) {
	if b {
		fmt.Println("Set master")
		AddMasterFilter(lnk, cfg)
		DelSlaveFilter(lnk, cfg)
	} else {
		fmt.Println("Set slave")
		DelMasterFilter(lnk, cfg)
		AddSlaveFilter(lnk, cfg)
	}
}

func AddMasterFilter(lnk netlink.Link, cfg *Config) {
	cmd := exec.Command(
		"tc", "filter", "add", "dev", lnk.Attrs().Name, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(cfg.Filter.Master.Priority), "u32",
		"match", "ip", "dst", cfg.Filter.Master.IP,
		"action", "nat", "ingress", cfg.Filter.Master.IP, cfg.Filter.Route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка добавление мастер-фильтра")
	}
}
func DelMasterFilter(lnk netlink.Link, cfg *Config) {
	cmd := exec.Command(
		"tc", "filter", "delete", "dev", lnk.Attrs().Name, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(cfg.Filter.Master.Priority), "u32",
		"match", "ip", "dst", cfg.Filter.Master.IP,
		"action", "nat", "ingress", cfg.Filter.Master.IP, cfg.Filter.Route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка удаления master-фильтра: filter does not exist")
	}
}
func AddSlaveFilter(lnk netlink.Link, cfg *Config) {
	cmd := exec.Command(
		"tc", "filter", "add", "dev", lnk.Attrs().Name, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(cfg.Filter.Slave.Priority), "u32",
		"match", "ip", "dst", cfg.Filter.Slave.IP,
		"action", "nat", "ingress", cfg.Filter.Slave.IP, cfg.Filter.Route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка добавления slave-фильтра")
	}
}
func DelSlaveFilter(lnk netlink.Link, cfg *Config) {
	cmd := exec.Command(
		"tc", "filter", "delete", "dev", lnk.Attrs().Name, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(cfg.Filter.Slave.Priority), "u32",
		"match", "ip", "dst", cfg.Filter.Slave.IP,
		"action", "nat", "ingress", cfg.Filter.Slave.IP, cfg.Filter.Route,
	)
	log.Println("Выполнение команды:", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка удаления slave-фильтра: filter does not exist")
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

func Route(lnk netlink.Link, ips ...string) error {
	for _, ip := range ips {
		ipParsed := net.ParseIP(ip)

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
