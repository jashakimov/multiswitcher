package filter

import "math/big"

type Cfg struct {
	Tries       int  `json:"tries"`
	SecToSwitch int  `json:"secToSwitch"`
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
	Bytes          *big.Int `json:"bytes"`
	Cfg            Cfg      `json:"config"`
}
