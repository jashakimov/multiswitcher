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
	turnOn       chan string
	turnOnF      chan *Filter
	turnOffF     chan *Filter
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
				return
			}

			if info.IsMasterActual && info.MasterBytes == nil {
				info.SetBytes(newBytes)
				continue

			}
			if !info.IsMasterActual && info.SlaveBytes == nil {
				info.SetBytes(newBytes)
				continue
			}

			oldBytes := info.GetBytes()
			//fmt.Println("Кол-во новых байт:", newBytes.String(), ", старых", oldBytes, "Попыток", tries, "ip", ip)
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
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()

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

	s.deleteIP(delIP)
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
			go func() {
				s.turnOnF <- data
			}()
		}
	}
}

func (s *service) autoRunner() {
	for {
		select {
		case f := <-s.turnOnF:
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
					//fmt.Println("Кол-во новых байт:", newBytes.String(), ", старых", oldBytes, "Попыток", tries, "ip", ip)
					if oldBytes.Cmp(newBytes) == 0 || oldBytes.Cmp(newBytes) > 0 {
						tries++
						if tries >= f.Cfg.Tries {

							f.IsMasterActual = !f.IsMasterActual
							s.turnOffF <- f
							f.SetBytes(nil)
							return
						}
					}
					f.SetBytes(newBytes)
				}
			}()

		case f := <-s.turnOffF:
			var (
				actualIP, changingIP     string
				actualPrio, changingPrio int
			)

			if f.IsMasterActual {
				actualIP = f.MasterIP
				changingIP = f.SlaveIP
				actualPrio = f.Cfg.MasterPrio
				changingPrio = f.Cfg.SlavePrio
			} else {
				actualIP = f.SlaveIP
				changingIP = f.MasterIP
				actualPrio = f.Cfg.SlavePrio
				changingPrio = f.Cfg.MasterPrio
			}

			log.Printf("Меняем с %s на %s \n", actualIP, changingIP)
			s.Del(f.InterfaceName, actualPrio, actualIP, f.DstIP)
			s.Add(f.InterfaceName, changingPrio, changingIP, f.DstIP)

			s.deleteIP(actualIP)

			s.turnOnF <- f
		}
	}
}
