package config

import (
	"encoding/json"
	"io"
	"os"
)

type Config struct {
	Interface        string   `json:"interface"`
	Port             string   `json:"port"`
	CopyTrafficFrom  string   `json:"copyTrafficFrom"`
	StatFrequencySec int      `json:"statsFrequencyMs"`
	Filters          []Filter `json:"filters"`
}

type Filter struct {
	Route       string `json:"route,omitempty"`
	SwitchTries int    `json:"switchTries,omitempty"`
	AutoSwitch  bool   `json:"autoSwitch"`
	Master      Info   `json:"master,omitempty"`
	Slave       Info   `json:"slave,omitempty"`
}

type Info struct {
	IP       string `json:"ip,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

func NewConfig(fileName string) *Config {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}

	bytes, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}

	var cfg Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		panic(err)
	}
	return &cfg
}
