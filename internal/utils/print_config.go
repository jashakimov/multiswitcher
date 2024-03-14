package utils

import (
	"fmt"
	"github.com/jashakimov/multiswitcher/internal/config"
)

func PrintConfig(cfg *config.Config) {

	fmt.Println("Total pairs:", len(cfg.Filters), "for Interface:", cfg.Interface)
	for i, filter := range cfg.Filters {
		fmt.Printf(" %d) masterIP: '%s', slaveIP: '%s', changeIP: '%s', tries before switch: '%d'\n",
			i+1,
			filter.Master.IP,
			filter.Slave.IP,
			filter.Route,
			filter.SwitchTries,
		)
	}
}
