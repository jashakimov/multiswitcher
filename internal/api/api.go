package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"github.com/jashakimov/multiswitcher/internal/utils"
	"net/http"
	"strconv"
	"strings"
)

func RegisterAPI(
	server *gin.Engine,
	db map[int]*filter.Filter,
	statService statistic.Service,
	filterService filter.Service,
) {
	s := &service{db: db, statService: statService, filterService: filterService}

	server.GET("/stats", s.getConfigs)
	server.GET("/stats/:id", s.getConfigByID)
	server.PATCH("/auto-switch/:id/:val", s.setAutoSwitch)
	server.PATCH("/switch/:id/:name", s.switchFilter)
}

type service struct {
	db            map[int]*filter.Filter
	statService   statistic.Service
	filterService filter.Service
}

func (s *service) getConfigs(ctx *gin.Context) {
	var filters []*filter.Filter
	for _, f := range s.db {
		filters = append(filters, f)
	}
	ctx.JSON(http.StatusOK, filters)
}

func (s *service) switchFilter(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, err.Error())
		return
	}
	name := strings.ToLower(ctx.Param("name"))
	if !utils.InSlice(name, []string{"master", "slave"}) {
		ctx.JSON(http.StatusBadRequest, "Значение только master/slave")
		return
	}

	filterInfo, ok := s.db[id]
	if !ok {
		ctx.JSON(http.StatusNotFound, "Не найден")
		return
	}

	switch {
	case name == "slave" && !filterInfo.IsMasterActual:
		ctx.JSON(http.StatusBadRequest, "Фильтр уже на slave")
		return
	case name == "master" && filterInfo.IsMasterActual:
		ctx.JSON(http.StatusBadRequest, "Фильтр уже на master")
		return
	}

	if name == "slave" {
		filterInfo.IsMasterActual = false

		if filterInfo.Cfg.AutoSwitch {
			s.filterService.TurnOffAutoSwitch(filterInfo.MasterIP)
			s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
			s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
			go s.filterService.TurnOnAutoSwitch(filterInfo)
		} else {
			s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
			s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
		}
	} else {
		filterInfo.IsMasterActual = true
		if filterInfo.Cfg.AutoSwitch {
			s.filterService.TurnOffAutoSwitch(filterInfo.SlaveIP)
			s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
			s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
			go s.filterService.TurnOnAutoSwitch(filterInfo)
		} else {
			s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
			s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
		}

	}

	ctx.JSON(http.StatusOK, filterInfo)
}

func (s *service) getConfigByID(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, err.Error())
		return
	}
	filterInfo, ok := s.db[id]
	if !ok {
		ctx.JSON(http.StatusNotFound, "Not found")
		return
	}

	ctx.JSON(http.StatusOK, filterInfo)
}

func (s *service) setAutoSwitch(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, err.Error())
		return
	}

	var autoSwitchVal bool
	switch strings.ToLower(ctx.Param("val")) {
	case "on":
		autoSwitchVal = true
	case "off":
		autoSwitchVal = false
	default:
		ctx.JSON(http.StatusBadRequest, "Параметр только on или off")
		return
	}

	filterInfo, ok := s.db[id]
	if !ok {
		ctx.JSON(http.StatusNotFound, "Не найден")
		return
	}

	filterInfo.Cfg.AutoSwitch = autoSwitchVal

	if autoSwitchVal {
		if filterInfo.IsMasterActual {
			go s.filterService.TurnOffAutoSwitch(filterInfo.SlaveIP)
			go s.filterService.TurnOnAutoSwitch(filterInfo)
		} else {
			go s.filterService.TurnOffAutoSwitch(filterInfo.MasterIP)
			go s.filterService.TurnOnAutoSwitch(filterInfo)
		}
	} else {
		go s.filterService.TurnOffAutoSwitch(filterInfo.SlaveIP)
		go s.filterService.TurnOffAutoSwitch(filterInfo.MasterIP)
	}

	ctx.JSON(http.StatusOK, filterInfo)
}
