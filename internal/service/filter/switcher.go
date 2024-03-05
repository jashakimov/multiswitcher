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
	IsExistFilters(data *Filter) (bool, bool)
	TurnOnAutoSwitch(f *Filter)
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

func (s *service) configureFilters(db map[int]*Filter) {
	for _, data := range db {

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
			go s.autoRunner()
			go s.TurnOnAutoSwitch(data)
		}
	}
}

func (s *service) TurnOnAutoSwitch(f *Filter) {
	s.turnOn <- f
}

func (s *service) TurnOffAutoSwitch(f *Filter) {
	s.turnOff <- f
}

func (s *service) autoRunner() {
	for {
		select {
		case f := <-s.turnOn:
			go func() {
				var (
					tries int
					ip    string
				)
				t := time.NewTicker(time.Duration(f.Cfg.SecToSwitch) * time.Millisecond)

				if f.IsMasterActual {
					s.addIP(f.MasterIP)
					ip = f.MasterIP
				} else {
					s.addIP(f.SlaveIP)
					ip = f.SlaveIP
				}
				log.Println("Включаем автопереключеине для ", ip)

				for range t.C {
					newBytes, err := s.statManager.GetBytesByIP(ip)
					if err != nil {
						log.Println(err)
						return
					}

					if f.IsMasterActual && f.MasterBytes == nil {
						f.SetBytes(newBytes)
						continue

					}
					if !f.IsMasterActual && f.SlaveBytes == nil {
						f.SetBytes(newBytes)
						continue
					}

					oldBytes := f.GetBytes()
					if oldBytes.Cmp(newBytes) == 0 || oldBytes.Cmp(newBytes) > 0 {
						tries++
						if tries >= f.Cfg.Tries {
							f.IsMasterActual = !f.IsMasterActual
							s.turnOff <- f
							f.SetBytes(nil)
							return
						}
					}
					f.SetBytes(newBytes)
				}
			}()

		case f := <-s.turnOff:
			s.switchFilters(f, false)

			if f.IsMasterActual {
				s.deleteIP(f.MasterIP)
			} else {
				s.deleteIP(f.SlaveIP)
			}

			if f.Cfg.AutoSwitch {
				s.turnOn <- f
			}
		}
	}
}

func (s *service) switchFilters(info *Filter, isAdd bool) {
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

	if isAdd {
		log.Println("Добавляем фильтр:", changingIP)
		s.Add(info.InterfaceName, changingPrio, changingIP, info.DstIP)
	} else {
		log.Println("Удаляем фильтр:", changingIP)
		s.Del(info.InterfaceName, actualPrio, actualIP, info.DstIP)
	}
}
