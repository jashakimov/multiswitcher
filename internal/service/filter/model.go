package filter

import "math/big"

type Cfg struct {
	Tries       int  `json:"tries,omitempty"`
	SecToSwitch int  `json:"secToSwitch,omitempty"`
	MasterPrio  int  `json:"-"`
	SlavePrio   int  `json:"-"`
	AutoSwitch  bool `json:"autoSwitch,omitempty"`
}

type Filter struct {
	Id             int      `json:"id,omitempty"`
	InterfaceName  string   `json:"interfaceName,omitempty"`
	MasterIP       string   `json:"masterIP,omitempty"`
	SlaveIP        string   `json:"slaveIP,omitempty"`
	DstIP          string   `json:"dstIP,omitempty"`
	IsMasterActual bool     `json:"isMasterActual,omitempty"`
	Bytes          *big.Int `json:"bytes,omitempty"`
	Cfg            Cfg      `json:"config,omitempty"`
}
