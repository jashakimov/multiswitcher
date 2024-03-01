package filter

import "math/big"

type Cfg struct {
	Tries       int  `json:"tries"`
	SecToSwitch int  `json:"msToSwitch"`
	MasterPrio  int  `json:"-"`
	SlavePrio   int  `json:"-"`
	AutoSwitch  bool `json:"autoSwitch"`
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

func (f *Filter) SetBytes(val *big.Int) {
	if f.IsMasterActual {
		f.MasterBytes = val
	} else {
		f.SlaveBytes = val
	}
}

func (f *Filter) GetBytes() *big.Int {
	if f.IsMasterActual {
		return f.MasterBytes
	}
	return f.SlaveBytes
}
