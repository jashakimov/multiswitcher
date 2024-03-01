package filter

import (
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type Service interface {
	Add(interfaceName string, priority int, ip, route string)
	Del(interfaceName string, priority int, ip, route string)
	TurnOnAutoSwitch(info *Filter)
	TurnOffAutoSwitch(ip string)
	IsExistFilters(data *Filter) (bool, bool)
}

type service struct {
	workersQueue map[string]struct{}
	turnOff      chan string
	statManager  statistic.Service
}

func NewService(statManager statistic.Service, db map[int]*Filter) Service {
	s := &service{
		turnOff:      make(chan string, 10),
		statManager:  statManager,
		workersQueue: make(map[string]struct{}),
	}
	s.configureFilters(db)

	return s
}

func (s *service) TurnOffAutoSwitch(ip string) {
	if _, ok := s.workersQueue[ip]; !ok {
		return
	}
	s.turnOff <- ip
}

func (s *service) Add(interfaceName string, priority int, ip, route string) {
	cmd := exec.Command(
		"tc", "filter", "add", "dev", interfaceName, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(priority), "u32",
		"match", "ip", "dst", ip,
		"action", "nat", "ingress", ip, route,
	)
	log.Println("Создание фильтра", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка добавление фильтра")
	}
}

func (s *service) Del(interfaceName string, priority int, ip, route string) {
	cmd := exec.Command(
		"tc", "filter", "delete", "dev", interfaceName, "parent", "ffff:",
		"protocol", "ip",
		"prio", strconv.Itoa(priority), "u32",
		"match", "ip", "dst", ip,
		"action", "nat", "ingress", ip, route,
	)
	log.Println("Удаление фильтра", cmd.String())
	if _, err := cmd.Output(); err != nil {
		log.Println("Ошибка удаления фильтра")
	}
}

func (s *service) TurnOnAutoSwitch(info *Filter) {
	var (
		tries int
		ip    string
	)
	t := time.NewTicker(time.Duration(info.Cfg.SecToSwitch) * time.Millisecond)

	if info.IsMasterActual {
		s.addIP(info.MasterIP)
		ip = info.MasterIP
	} else {
		s.addIP(info.SlaveIP)
		ip = info.SlaveIP
	}

	for {
		select {
		case ip := <-s.turnOff:
			s.deleteIP(ip)
			return

		case <-t.C:
			bytes, err := s.statManager.GetBytesByIP(ip)
			if err != nil {
				log.Println(err)
				continue
			}
			if info.Bytes == nil {
				info.Bytes = bytes
				continue
			}

			if info.Bytes.Cmp(bytes) == 0 || info.Bytes.Cmp(bytes) > 0 {
				tries++
				if tries >= info.Cfg.Tries {
					s.switchAndRestart(info, ip)
					return
				}
			}
			tries = 0
			info.Bytes = bytes
		}
	}
}

func (s *service) addIP(ip string) {
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()

	s.workersQueue[ip] = struct{}{}
}

func (s *service) deleteIP(ip string) {
	delete(s.workersQueue, ip)
}

func (s *service) switchAndRestart(info *Filter, delIP string) {
	var (
		masterIP, slaveIP     string
		masterPrio, slavePrio int
	)

	if info.IsMasterActual {
		masterIP = info.MasterIP
		slaveIP = info.SlaveIP
		masterPrio = info.Cfg.MasterPrio
		slavePrio = info.Cfg.SlavePrio
	} else {
		masterIP = info.SlaveIP
		slaveIP = info.MasterIP
		masterPrio = info.Cfg.SlavePrio
		slavePrio = info.Cfg.MasterPrio
	}

	s.Del(info.InterfaceName, masterPrio, masterIP, info.DstIP)
	s.Add(info.InterfaceName, slavePrio, slaveIP, info.DstIP)
	info.IsMasterActual = !info.IsMasterActual

	s.deleteIP(delIP)

	if info.Cfg.AutoSwitch {
		go s.TurnOnAutoSwitch(info)
	}
}

func (s *service) IsExistFilters(data *Filter) (bool, bool) {
	_, masterErr := s.statManager.GetBytesByIP(data.MasterIP)
	_, slaveErr := s.statManager.GetBytesByIP(data.SlaveIP)
	return masterErr == nil, slaveErr == nil
}

func (s *service) configureFilters(db map[int]*Filter) {
	for _, data := range db {
		//удаляем старые фильтры
		//filter.Del(data.InterfaceName, data.Cfg.MasterPrio, data.MasterIP, data.DstIP)
		//filter.Del(data.InterfaceName, data.Cfg.SlavePrio, data.SlaveIP, data.DstIP)

		// проверяем текущие фильтры
		isMaster, isSlave := s.IsExistFilters(data)
		switch {
		case isSlave:
			data.IsMasterActual = false
		case isMaster:
			data.IsMasterActual = true
			// установка мастер фильтров по умолчанию
		case !isSlave && !isMaster:
			data.IsMasterActual = true
			s.Add(data.InterfaceName, data.Cfg.MasterPrio, data.MasterIP, data.DstIP)
		}

		// Запускаем воркер на переключение слейв
		if data.Cfg.AutoSwitch {
			go s.TurnOnAutoSwitch(data)
		}
	}
}
