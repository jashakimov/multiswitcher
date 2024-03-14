package interface_link

import (
	"fmt"
	"github.com/jashakimov/multiswitcher/internal/config"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os/exec"
	"strconv"
)

func Configure(link netlink.Link, cfg *config.Config) {
	log.Printf("Установка мультикаста на интерфейс: %s\n", cfg.Interface)
	// установка мултикаста
	if err := LinkSetMulticast(link); err != nil {
		panic(err)
	}

	log.Printf("Установка промискуитетного режима на интерфейс: %s\n", cfg.Interface)
	// установка промискуитетного режима
	if err := netlink.SetPromiscOn(link); err != nil {
		panic(err)
	}

	log.Printf("Установка qdisc на интерфейс: %s", cfg.Interface)
	// установка дисциплины, для последующей установки фильтров
	if err := SetIngressQDisc(link); err != nil {
		fmt.Println(err)
	}

	// установка маршрутизации роутеров
	if err := Route(link, cfg.Filters); err != nil {
		panic(err)
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

func Route(lnk netlink.Link, filters []config.Filter) error {
	for _, f := range filters {
		ipParsed := net.ParseIP(f.Route)

		route := &netlink.Route{
			Dst: &net.IPNet{
				IP:   ipParsed,
				Mask: net.CIDRMask(32, 32),
			},
			LinkIndex: lnk.Attrs().Index,
		}

		if _, err := netlink.RouteGet(ipParsed); err == nil {
			continue
		}

		if err := netlink.RouteAdd(route); err != nil {
			return err
		}
		log.Printf("Установка маршрутизации %s на интерфейс: %s\n", f.Route, lnk.Attrs().Name)
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

func MirrorTraffic(from, to netlink.Link, ips map[int]*filter.Filter) {
	for _, ip := range ips {
		cmdMaster := exec.Command("tc", "filter", "add", "dev",
			from.Attrs().Name, "parent", "ffff:", "protocol", "ip", "prio", strconv.Itoa(ip.Cfg.MasterPrio), "u32",
			"match", "ip", "dst", ip.MasterIP+"/32", "action", "mirred", "egress", "mirror", "dev", to.Attrs().Name)
		cmdMaster.CombinedOutput()
		cmdSlave := exec.Command("tc", "filter", "add", "dev",
			from.Attrs().Name, "parent", "ffff:", "protocol", "ip", "prio", strconv.Itoa(ip.Cfg.SlavePrio), "u32",
			"match", "ip", "dst", ip.SlaveIP+"/32", "action", "mirred", "egress", "mirror", "dev", to.Attrs().Name)
		cmdSlave.CombinedOutput()
	}
}
