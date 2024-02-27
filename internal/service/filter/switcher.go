package filter

import (
	"fmt"
	"github.com/jashakimov/multiswitcher/internal/service/statistic"
	"log"
	"os/exec"
	"strconv"
	"time"
)

func Add(interfaceName string, priority int, ip, route string) {
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
func Del(interfaceName string, priority int, ip, route string) {
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

func SwitchToMaster(statManager statistic.Service, info Filter) {
	var tries int
	t := time.NewTicker(time.Second)
	for range t.C {
		bytes, err := statManager.GetBytesByIP(info.MasterIP)
		if err != nil {
			log.Println(err)
			continue
		}

		if info.Bytes.Cmp(bytes) == 0 || info.Bytes.Cmp(bytes) > 0 {
			tries++
			if tries >= info.Cfg.Tries {
				fmt.Println("Переключение на слейв")
				Del(info.InterfaceName, info.Cfg.MasterPrio, info.MasterIP, info.DstIP)
				Add(info.InterfaceName, info.Cfg.SlavePrio, info.SlaveIP, info.DstIP)
				return
			}
		}
		info.Bytes = bytes
	}
}
