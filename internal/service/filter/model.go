package filter

import (
	"log"
	"math/big"
	"sync"
)

type Cfg struct {
	Tries      int  `json:"tries"`
	MsToSwitch int  `json:"msToSwitch"`
	MasterPrio int  `json:"-"`
	SlavePrio  int  `json:"-"`
	AutoSwitch bool `json:"autoSwitch"`
}

type Filter struct {
	Id             int      `json:"id"`
	InterfaceName  string   `json:"interfaceName"`
	MasterIP       string   `json:"masterIP"`
	SlaveIP        string   `json:"slaveIP"`
	DstIP          string   `json:"dstIP"`
	IsMasterActual bool     `json:"isMasterActual"`
	MasterBytes    *big.Int `json:"masterBytes"`
	SlaveBytes     *big.Int `json:"slaveBytes"`
	Cfg            Cfg      `json:"config"`
}

var mu sync.Mutex

func (f *Filter) SetBytes(val *big.Int) {
	mu.Lock()
	defer mu.Unlock()
	var ip string

	if f.IsMasterActual {
		f.MasterBytes = val
		ip = f.MasterIP
	} else {
		f.SlaveBytes = val
		ip = f.SlaveIP
	}
	log.Printf("Новое значение для %s, %s", ip, val.String())
}

func (f *Filter) GetBytes() *big.Int {
	if f.IsMasterActual {
		return f.MasterBytes
	}
	return f.SlaveBytes
}

func (f *Filter) GetActualIP() string {
	if f.IsMasterActual {
		return f.MasterIP
	}
	return f.SlaveIP
}
