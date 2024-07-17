package main

import (
	"github.com/gin-gonic/gin"
	_ "github.com/google/gopacket/layers"
	"github.com/jashakimov/multiswitcher/internal/api"
	"github.com/jashakimov/multiswitcher/internal/config"
	"github.com/jashakimov/multiswitcher/internal/interface_link"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/jashakimov/multiswitcher/internal/service/igmp"
	"github.com/jashakimov/multiswitcher/internal/service/net_listener"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"github.com/jashakimov/multiswitcher/internal/utils"
	"github.com/vishvananda/netlink"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var Version string

func main() {
	fileConfig := utils.ParseFlags()
	cfg := config.NewConfig(fileConfig)
	log.Println("Версия приложения:", Version)

	link, err := netlink.LinkByName(cfg.Interface)
	if err != nil {
		panic(err)
	}
	copyFrom, err := netlink.LinkByName(cfg.CopyTrafficFrom)
	if err != nil {
		panic(err)
	}

	db := MakeLocalDB(cfg)
	interface_link.SetIngressQDisc(copyFrom)
	interface_link.MirrorTraffic(copyFrom, link, db)
	interface_link.Configure(link, cfg)
	statManager := statistic.NewService(link.Attrs().Name, cfg.StatFrequencySec)
	netListener := net_listener.NewService(cfg.Interface)
	filterManager := filter.NewService(statManager, db, netListener)
	imgpService := igmp.NewService(db)

	gin.SetMode(gin.ReleaseMode)
	server := gin.New()
	server.Use(gin.Recovery(), gin.Logger())
	api.RegisterAPI(server, db, filterManager, imgpService)

	go func() {
		log.Println("Запущен сервер, порт", cfg.Port)
		if err := server.Run(":" + cfg.Port); err != nil {
			panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func MakeLocalDB(cfg *config.Config) map[int]*filter.Filter {
	info := make(map[int]*filter.Filter)
	for i, f := range cfg.Filters {

		info[i+1] = &filter.Filter{
			Id:                i + 1,
			InterfaceName:     cfg.Interface,
			MasterIP:          f.Master.IP,
			Hostname:          cfg.Hostname,
			SlaveIP:           f.Slave.IP,
			DstIP:             f.Route,
			Title:             f.Title,
			IsMasterActual:    false,
			IsIgmpOn:          false,
			IsReturnToMaster:  false,
			MasterBytes:       nil,
			SlaveBytes:        nil,
			CopyFromInterface: cfg.CopyTrafficFrom,
			Cfg: filter.Cfg{
				Tries:      f.SwitchTries,
				MsToSwitch: cfg.StatFrequencySec,
				MasterPrio: i + 1,
				SlavePrio:  i + 1,
				AutoSwitch: f.AutoSwitch,
			},
		}
	}

	return info
}
