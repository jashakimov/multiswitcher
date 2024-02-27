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
}

type service struct {
	cache  *utils.SyncMap[string, *big.Int]
	cmd    *exec.Cmd
	regexp *regexp.Regexp
}

func NewService(linkName string) Service {
	s := &service{
		cache: utils.NewSyncMap[string, *big.Int](),
	}
	s.cmd = exec.Command("tc", "-s", "-pretty", "filter", "show", "ingress", "dev", linkName)
	// нам нужна инфа в скобках (match[1] и match[2])
	s.regexp = regexp.MustCompile(`dst (\S+)/\S+\n.+\n.+\n.+\n.+Sent (\d+)`)

	go s.readStats()

	return s
}

func (s *service) GetBytesByIP(ip string) (*big.Int, error) {
	if bytes, ok := s.cache.Get(ip); ok {
		return bytes, nil
	}
	return nil, errors.Newf("Uknown IP: %s\n", ip)
}

func (s *service) readStats() {
	t := time.NewTicker(time.Second)

	for range t.C {
		statsOutput, err := s.cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}

		matches := s.regexp.FindAllStringSubmatch(string(statsOutput), -1)
		for _, match := range matches {
			bytes := new(big.Int)
			bytes.SetString(match[2], 10)

			s.cache.Set(match[1], bytes)
		}
	}
}
