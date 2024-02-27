package utils

import (
	"flag"
)

func ParseFlags() string {
	var fileConfig string
	flag.StringVar(&fileConfig, "config", "", "path to config file")
	flag.Parse()
	if fileConfig == "" {
		panic("No file directory")
	}
	return fileConfig
}
