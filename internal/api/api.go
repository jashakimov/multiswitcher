package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
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

func NewService(db map[int]*filter.Filter, statService statistic.Service) Service {
	return &service{db: db, statService: statService}
}

type service struct {
	db          map[int]*filter.Filter
	statService statistic.Service
}

func (s *service) GetConfigs(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, s.db)
}

func (s *service) Switch(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	name := ctx.Param("name")
	if name != "slave" {
		ctx.String(http.StatusBadRequest, "Переключение на slave/master")
		return
	}
	if name != "master" {
		ctx.String(http.StatusBadRequest, "Переключение на slave/master")
		return
	}

	filterInfo, ok := s.db[id]
	if !ok {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	// переключать можем, если только автопереключение выключено
	if !filterInfo.Cfg.AutoSwitch {
		if filterInfo.IsMasterActual {
			filter.Del(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
			filter.Add(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
		} else {
			filter.Del(filterInfo.InterfaceName, filterInfo.Cfg.SlavePrio, filterInfo.SlaveIP, filterInfo.DstIP)
			filter.Add(filterInfo.InterfaceName, filterInfo.Cfg.MasterPrio, filterInfo.MasterIP, filterInfo.DstIP)
		}
	}
}

func (s *service) GetConfigByID(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	filterInfo, ok := s.db[id]
	if !ok {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	ctx.JSON(http.StatusOK, filterInfo)
}

func (s *service) SetAutoSwitch(ctx *gin.Context) {
	rawId := ctx.Param("id")
	id, err := strconv.Atoi(rawId)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	var autoSwitchVal bool
	switch strings.ToLower(ctx.Param("val")) {
	case "on":
		autoSwitchVal = true
	case "off":
		autoSwitchVal = false
	default:
		ctx.String(http.StatusBadRequest, "Параметр только on или off")
		return
	}

	filterInfo, ok := s.db[id]
	if !ok {
		ctx.String(http.StatusNotFound, "Не найден")
		return
	}

	filterInfo.Cfg.AutoSwitch = autoSwitchVal
	if autoSwitchVal {
		go filter.TurnOnAutoSwitch(s.statService, filterInfo)
	}

	ctx.JSON(http.StatusOK, filterInfo)
}
