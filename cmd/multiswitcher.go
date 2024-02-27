package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/google/gopacket/layers"
	"github.com/jashakimov/multiswitcher/internal/api"
	"github.com/jashakimov/multiswitcher/internal/config"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"github.com/jashakimov/multiswitcher/internal/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	fileConfig := utils.ParseFlags()
	cfg := config.NewConfig(fileConfig)

	link, err := netlink.LinkByName(cfg.Interface)
	if err != nil {
		panic(err)
	}

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

	log.Printf("Установка qdisc на интерфейс: %s\n", cfg.Interface)
	// установка дисциплины, для последующей установки фильтров
	if err := SetIngressQDisc(link); err != nil {
		fmt.Println(err)
	}

	// установка маршрутизации роутеров
	if err := Route(link, cfg.Filters); err != nil {
		panic(err)
	}

	db := MakeLocalDB(cfg)
	statManager := statistic.NewService(link.Attrs().Name)
	CreateFilters(db, statManager)

	api := api.NewService(db, statManager)
	server := gin.Default()

	server.GET("/stats", api.GetConfigs)
	server.GET("/stats/:id", api.GetConfigByID)
	server.PATCH("/auto-switch/:id/:val", api.SetAutoSwitch)
	server.PATCH("/switch/:id/:name", api.Switch)

	go func() {
		log.Println("Запущен сервер, порт", cfg.Port)
		if err := server.Run(cfg.Port); err != nil {
			panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func CreateFilters(db map[int]filter.Filter, statService statistic.Service) {
	for _, data := range db {
		//удаляем старые фильтры
		filter.Del(data.InterfaceName, data.Cfg.MasterPrio, data.MasterIP, data.DstIP)
		filter.Del(data.InterfaceName, data.Cfg.SlavePrio, data.SlaveIP, data.DstIP)
		// установка мастер фильтров по умолчанию
		filter.Add(data.InterfaceName, data.Cfg.MasterPrio, data.MasterIP, data.DstIP)

		// Запускаем воркер на переключение слейв
		if data.Cfg.AutoSwitch {
			go filter.TurnOnAutoSwitch(statService, data)
		}
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

func MakeLocalDB(cfg *config.Config) map[int]filter.Filter {
	info := make(map[int]filter.Filter)
	for i, f := range cfg.Filters {
		info[i+1] = filter.Filter{
			Id:             i + 1,
			InterfaceName:  cfg.Interface,
			MasterIP:       f.Master.IP,
			SlaveIP:        f.Slave.IP,
			DstIP:          f.Route,
			IsMasterActual: true,
			Bytes:          nil,
			Cfg: filter.Cfg{
				Tries:       f.SwitchTries,
				SecToSwitch: f.StatFrequencySec,
				MasterPrio:  f.Master.Priority,
				SlavePrio:   f.Slave.Priority,
				AutoSwitch:  f.AutoSwitch,
			},
		}
	}
	return info
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
