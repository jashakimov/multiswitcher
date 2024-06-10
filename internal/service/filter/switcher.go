package filter

import (
	"github.com/jashakimov/multiswitcher/internal/service/net_listener"
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
	ReturnToMaster(info *Filter, toggle bool)
}

type service struct {
	workersQueue           map[string]struct{}
	turnOff                chan *Filter
	statManager            statistic.Service
	listener               net_listener.Listener
	db                     map[int]*Filter
	returnToMasterChannels map[string]chan int
}

func NewService(statManager statistic.Service, db map[int]*Filter, listener net_listener.Listener) Service {
	s := &service{
		turnOff:      make(chan *Filter),
		statManager:  statManager,
		workersQueue: make(map[string]struct{}),
		listener:     listener,
		db:           db,
	}
	s.configureFilters(db)
	s.returnToMasterListener()

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
	log.Println("Удаляем из очереди", ip)
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
	time.Sleep(time.Second * 2)
	for _, data := range db {
		// проверяем текущие фильтры
		isMaster, isSlave := s.IsExistFilters(data)
		log.Println("Exist filters", isMaster, isSlave)

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

		//инициализация каналов для прослушки мастер ip
		s.returnToMasterChannels[data.MasterIP] = make(chan int)
	}
}

func (s *service) addBytes(f *Filter) {
	t := time.NewTicker(time.Duration(f.Cfg.MsToSwitch) * time.Millisecond)
	for range t.C {
		actualIP := f.GetActualIP()
		bytes, err := s.statManager.GetBytesByIP(actualIP)
		if err != nil {
			continue
		}
		f.SetBytes(bytes)
	}
}

func (s *service) AutoSwitch(f *Filter) {
	var tries int

	t := time.NewTicker(time.Duration(f.Cfg.MsToSwitch) * time.Millisecond)
	for range t.C {
		actualIP := f.GetActualIP()
		bytes, err := s.statManager.GetBytesByIP(actualIP)

		if err != nil {
			log.Println(err)
			continue
		}
		if f.GetBytes() == nil {
			f.SetBytes(bytes)
			continue
		}
		// если количество новых байтов не изменилось
		if f.GetBytes().Cmp(bytes) == 0 && f.Cfg.AutoSwitch {
			tries++
			if tries >= f.Cfg.Tries {
				f.SetBytes(nil)
				s.ChangeFilter(f)
				f.IsMasterActual = !f.IsMasterActual
				s.statManager.DelBytesByIP(actualIP)
				s.deleteIP(actualIP)
				go s.AutoSwitch(f)
				return
			}
		} else {
			tries = 0
		}
		f.SetBytes(bytes)
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
	time.Sleep(250 * time.Millisecond)
}

func (s *service) ReturnToMaster(info *Filter, toggleOn bool) {
	// если false, то выключить возврат на мастер
	if toggleOn {
		info.IsReturnToMaster = true
		receiveChan, ok := s.returnToMasterChannels[info.MasterIP]
		if !ok {
			panic("Нет канала для возврата на мастер для " + info.MasterIP)
		}
		s.listener.Receive(info.MasterIP, net_listener.Info{
			Id:          info.Id,
			ReceiveChan: receiveChan,
		})
	} else {
		s.listener.Stop(info.MasterIP)
		info.IsReturnToMaster = false
	}
}

func (s *service) returnToMasterListener() {
	for _, ch := range s.returnToMasterChannels {
		go func(c chan int) {
			for filterId := range c {
				if fil, ok := s.db[filterId]; ok {
					if !fil.IsMasterActual {
						s.ChangeFilter(fil)
						fil.IsMasterActual = !fil.IsMasterActual
					}
				}
			}
		}(ch)
	}
}
