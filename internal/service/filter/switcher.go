package filter

import (
	"fmt"
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
	log.Println("Включаем автопереключеине для ", ip)

	for {
		select {
		case ip := <-s.turnOff:
			log.Println("Отключаем автопереключеине для ", ip)
			if _, ok := s.workersQueue[ip]; ok {
				s.deleteIP(ip)
				return
			}
		case <-t.C:
			newBytes, err := s.statManager.GetBytesByIP(ip)
			if err != nil {
				log.Println(err)
				continue
			}

			if info.IsMasterActual && info.MasterBytes == nil {
				info.MasterBytes = newBytes
				continue

			}
			if !info.IsMasterActual && info.SlaveBytes == nil {
				info.SlaveBytes = newBytes
				continue
			}

			oldBytes := info.GetBytes()
			fmt.Println("Кол-во новых байт:", newBytes.String(), ", старых", info.MasterBytes, "Попыток", tries)
			if oldBytes.Cmp(newBytes) == 0 || oldBytes.Cmp(newBytes) > 0 {
				tries++
				if tries >= info.Cfg.Tries {
					s.switchAndRestart(info, ip)
					info.SetBytes(nil)
					return
				}
			}
			info.SetBytes(newBytes)
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
		actualIP, changingIP     string
		actualPrio, changingPrio int
	)

	if info.IsMasterActual {
		actualIP = info.MasterIP
		changingIP = info.SlaveIP
		actualPrio = info.Cfg.MasterPrio
		changingPrio = info.Cfg.SlavePrio
	} else {
		actualIP = info.SlaveIP
		changingIP = info.MasterIP
		actualPrio = info.Cfg.SlavePrio
		changingPrio = info.Cfg.MasterPrio
	}

	log.Printf("Меняем с %s на %s \n", actualIP, changingIP)
	s.Del(info.InterfaceName, actualPrio, actualIP, info.DstIP)
	s.Add(info.InterfaceName, changingPrio, changingIP, info.DstIP)
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
