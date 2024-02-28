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

type Service interface {
	GetConfigByID(ctx *gin.Context)
	GetConfigs(ctx *gin.Context)
	SetAutoSwitch(ctx *gin.Context)
	Switch(ctx *gin.Context)
}

func NewService(
	db map[int]*filter.Filter,
	statService statistic.Service,
	filterService filter.Service,
) Service {
	return &service{db: db, statService: statService, filterService: filterService}
}

type service struct {
	db            map[int]*filter.Filter
	statService   statistic.Service
	filterService filter.Service
}

func (s *service) GetConfigs(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, s.db)
}

func (s *service) Switch(ctx *gin.Context) {
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

	// переключать можем, если только автопереключение выключено
	if filterInfo.Cfg.AutoSwitch {
		ctx.JSON(http.StatusNotFound, "Переключить можно, если автопереключение выключено")
		return
	}

	if name == "slave" {
		s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
		s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
		filterInfo.IsMasterActual = false
	} else {
		s.filterService.Del(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
		s.filterService.Add(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
		filterInfo.IsMasterActual = true
	}
}

func (s *service) GetConfigByID(ctx *gin.Context) {
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

func (s *service) SetAutoSwitch(ctx *gin.Context) {
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
			go s.filterService.TurnOnAutoSwitch(filterInfo)
		}
	} else {
		s.filterService.TurnOffAutoSwitch(filterInfo.MasterIP)
	}

	ctx.JSON(http.StatusOK, filterInfo)
}
