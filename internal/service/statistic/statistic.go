package statistic

import (
	"github.com/jashakimov/multiswitcher/internal/utils"
	"gopkg.in/errgo.v2/fmt/errors"
	"math/big"
	"os/exec"
	"regexp"
	"time"
)

type Service interface {
	GetBytesByIP(ip string) (*big.Int, error)
	DelBytesByIP(ip string)
}

type service struct {
	cache         *utils.SyncMap[string, *big.Int]
	interfaceName string
}

func NewService(linkName string, timeoutMs int) Service {
	s := &service{
		interfaceName: linkName,
		cache:         utils.NewSyncMap[string, *big.Int](),
	}

	go s.readStats(timeoutMs)
	time.Sleep(time.Second)

	return s
}

func (s *service) GetBytesByIP(ip string) (*big.Int, error) {
	if bytes, ok := s.cache.Get(ip); ok {
		return bytes, nil
	}
	return nil, errors.Newf("Uknown IP: %s\n", ip)
}

func (s *service) DelBytesByIP(ip string) {
	s.cache.Del(ip)
}

func (s *service) readStats(timeoutMs int) {
	t := time.NewTicker(time.Duration(timeoutMs) * time.Millisecond)

	for range t.C {
		cmd := exec.Command("tc", "-s", "-pretty", "filter", "show", "ingress", "dev", s.interfaceName)
		// нам нужна инфа в скобках (match[1] и match[2])
		reg := regexp.MustCompile(`dst (\S+)/\S+\n.+\n.+\n.+\n.+Sent (\d+)`)

		statsOutput, err := cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}

		matches := reg.FindAllStringSubmatch(string(statsOutput), -1)

		for _, match := range matches {
			bytes := new(big.Int)
			bytes.SetString(match[2], 10)
			s.cache.Set(match[1], bytes)
		}
	}
}
