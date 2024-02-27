package filter

import "math/big"

type Cfg struct {
	Tries       int
	SecToSwitch int
	MasterPrio  int
	SlavePrio   int
	AutoSwitch  bool
}

type Filter struct {
	Id             int
	InterfaceName  string   `json:"interfaceName,omitempty"`
	MasterIP       string   `json:"masterIP,omitempty"`
	SlaveIP        string   `json:"slaveIP,omitempty"`
	DstIP          string   `json:"dstIP,omitempty"`
	IsMasterActual bool     `json:"isMasterActual,omitempty"`
	Bytes          *big.Int `json:"bytes,omitempty"`
	Cfg            Cfg
}
