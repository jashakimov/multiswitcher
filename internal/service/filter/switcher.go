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
	IsExistFilters(data *Filter) (bool, bool)
	AutoSwitch(f *Filter)
	ChangeFilter(f *Filter)
	TurnOffAutoSwitch(f *Filter)
}

type service struct {
	workersQueue map[string]struct{}
	turnOff      chan *Filter
	turnOn       chan *Filter
	statManager  statistic.Service
}

func NewService(statManager statistic.Service, db map[int]*Filter) Service {
	s := &service{
		turnOff:      make(chan *Filter),
		statManager:  statManager,
		workersQueue: make(map[string]struct{}),
	}
	s.configureFilters(db)

	return s
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

func (s *service) addIP(ip string) {
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()

	s.workersQueue[ip] = struct{}{}
}

func (s *service) deleteIP(ip string) {
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()

	delete(s.workersQueue, ip)
}

func (s *service) IsExistFilters(data *Filter) (bool, bool) {
	_, masterErr := s.statManager.GetBytesByIP(data.MasterIP)
	_, slaveErr := s.statManager.GetBytesByIP(data.SlaveIP)
	return masterErr == nil, slaveErr == nil
}

func (s *service) TurnOffAutoSwitch(f *Filter) {
	s.turnOff <- f
}

func (s *service) configureFilters(db map[int]*Filter) {
	for _, data := range db {

		// проверяем текущие фильтры
		time.Sleep(time.Second)
		isMaster, isSlave := s.IsExistFilters(data)
		fmt.Println("Фильтр мастера", isMaster, "Фильтр слейва", isSlave)
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

		go s.AutoSwitch(data)
	}
}

func (s *service) AutoSwitch(f *Filter) {
	var tries int
	actualIP := f.GetActualIP()
	if _, ok := s.workersQueue[actualIP]; ok {
		return
	}

	s.addIP(actualIP)

	t := time.NewTicker(time.Duration(f.Cfg.SecToSwitch) * time.Millisecond)
	for {
		select {
		case filter := <-s.turnOff:
			ip := filter.GetActualIP()
			if _, ok := s.workersQueue[ip]; ok {
				s.deleteIP(ip)
				return
			}
		case <-t.C:
			if !f.Cfg.AutoSwitch {
				return
			}
			bytes, err := s.statManager.GetBytesByIP(actualIP)
			fmt.Println("Количество байтов из мастер=", f.IsMasterActual)
			if err != nil {
				log.Println(err)
				return
			}
			if f.GetBytes() == nil {
				f.SetBytes(bytes)
				continue
			}
			// если количество новых байтов не изменилось
			if f.GetBytes().Cmp(bytes) == 0 {
				tries++
				fmt.Println("Статус мастера", f.IsMasterActual)
				if tries >= f.Cfg.Tries {
					fmt.Println("Было попыток", tries)
					f.SetBytes(nil)
					s.ChangeFilter(f)
					f.IsMasterActual = !f.IsMasterActual
					s.deleteIP(actualIP)
					fmt.Println("Статус мастера", f.IsMasterActual)
					go s.AutoSwitch(f)
					return
				}
			} else {
				tries = 0
				f.SetBytes(bytes)
			}
		}
	}
}

func (s *service) ChangeFilter(f *Filter) {
	var actualIP, newIP string
	var actualPrio, newPrio int

	if f.IsMasterActual {
		actualIP = f.MasterIP
		actualPrio = f.Cfg.MasterPrio

		newIP = f.SlaveIP
		newPrio = f.Cfg.SlavePrio
	} else {
		actualIP = f.SlaveIP
		actualPrio = f.Cfg.SlavePrio

		newIP = f.MasterIP
		newPrio = f.Cfg.MasterPrio
	}

	s.Del(f.InterfaceName, actualPrio, actualIP, f.DstIP)
	s.Add(f.InterfaceName, newPrio, newIP, f.DstIP)
}
