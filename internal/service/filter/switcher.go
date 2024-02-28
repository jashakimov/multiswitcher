package filter

import (
	"fmt"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"log"
	"os/exec"
	"strconv"
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
	fmt.Println("Записали в канал для отключения", ip)
	if _, ok := s.workersQueue[ip]; !ok {
		fmt.Println("Пусто", ip)
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
	log.Println("Включение авто-переключения при падении мастер-потока", info.MasterIP)

	s.workersQueue[info.MasterIP] = struct{}{}

	for {
		select {
		case ip := <-s.turnOff:
			log.Println("Выключение автоматического переключение", ip)
			delete(s.workersQueue, ip)
			return
		case <-t.C:
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
					fmt.Println("Переключение на слейв")
					s.Del(info.InterfaceName, info.Cfg.MasterPrio, info.MasterIP, info.DstIP)
					s.Add(info.InterfaceName, info.Cfg.SlavePrio, info.SlaveIP, info.DstIP)
					info.IsMasterActual = false
					return
				}
			}
			info.Bytes = bytes
		}
	}
}
