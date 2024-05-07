package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"github.com/jashakimov/multiswitcher/internal/utils"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func RegisterAPI(
	server *gin.Engine,
	db map[int]*filter.Filter,
	statService statistic.Service,
	filterService filter.Service,
	igmpService igmp.Service,
) {
	s := &service{db: db, statService: statService, filterService: filterService}

	server.GET("/stats", s.getConfigs)
	server.GET("/stats/:id", s.getConfigByID)
	server.PATCH("/auto-switch/:id/:val", s.setAutoSwitch)
	server.PATCH("/switch/:id/:name", s.switchFilter)
	server.PATCH("/igmp/:toggle", s.switchIgmp)
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
	sort.Slice(filters, func(i, j int) bool {
		return filters[i].Id < filters[j].Id
	})
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

	s.filterService.ChangeFilter(filterInfo)
	filterInfo.IsMasterActual = !filterInfo.IsMasterActual

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

	if filterInfo.Cfg.AutoSwitch {
		go s.filterService.AutoSwitch(filterInfo)
	}

	ctx.JSON(http.StatusOK, filterInfo)
}

func (s *service) switchIgmp(ctx *gin.Context) {
	var toogle bool
	switch strings.ToLower(ctx.Param("toggle")) {
	case "on":
		toogle = true
	case "off":
		toogle = false
	default:
		ctx.JSON(http.StatusBadRequest, "Параметр только on или off")
		return
	}

}
