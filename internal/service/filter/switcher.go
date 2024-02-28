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
}

type service struct {
	workersQueue map[string]struct{}
	turnOff      chan string
	statManager  statistic.Service
}

func NewService(statManager statistic.Service) Service {
	return &service{
		turnOff:      make(chan string, 10),
		statManager:  statManager,
		workersQueue: make(map[string]struct{}),
	}
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
	var tries int
	t := time.NewTicker(time.Duration(info.Cfg.SecToSwitch) * time.Millisecond)

	if info.IsMasterActual {
		s.addQueue(info.MasterIP)
	} else {
		s.addQueue(info.SlaveIP)
	}

	for {
		select {
		case ip := <-s.turnOff:
			delete(s.workersQueue, ip)
			return
		case <-t.C:
			if info.IsMasterActual {
				bytes, err := s.statManager.GetBytesByIP(info.MasterIP)
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
						s.Del(info.InterfaceName, info.Cfg.MasterPrio, info.MasterIP, info.DstIP)
						s.Add(info.InterfaceName, info.Cfg.SlavePrio, info.SlaveIP, info.DstIP)
						info.IsMasterActual = false

						s.delQueue(info.MasterIP)

						if info.Cfg.AutoSwitch {
							go s.TurnOnAutoSwitch(info)
						}

						return
					}
				}
				info.Bytes = bytes
			} else {
				bytes, err := s.statManager.GetBytesByIP(info.SlaveIP)
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
						s.Del(info.InterfaceName, info.Cfg.SlavePrio, info.SlaveIP, info.DstIP)
						s.Add(info.InterfaceName, info.Cfg.MasterPrio, info.MasterIP, info.DstIP)
						info.IsMasterActual = true

						s.delQueue(info.SlaveIP)

						if info.Cfg.AutoSwitch {
							go s.TurnOnAutoSwitch(info)
						}

						return
					}
				}
				info.Bytes = bytes
			}
		}
	}
}

func (s *service) addQueue(ip string) {
	var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()

	s.workersQueue[ip] = struct{}{}
}

func (s *service) delQueue(ip string) {
	delete(s.workersQueue, ip)
}
