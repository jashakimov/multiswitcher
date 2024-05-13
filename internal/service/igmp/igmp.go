package igmp

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/jashakimov/multiswitcher/internal/service/filter"
	"gopkg.in/errgo.v2/fmt/errors"
	"net"
	"time"
)

const JoinReport = 0x16
const LeaveGroup = 0x17

type Service interface {
	ToggleAll(ctx context.Context, msg byte)
	ToggleByID(ctx context.Context, id int, msg byte) error
}

type service struct {
	db                        map[int]*filter.Filter
	workingPool               map[int]Connection
	stopSendingJoinPeportChan chan int
}

func NewService(db map[int]*filter.Filter) Service {
	return &service{
		db:                        db,
		workingPool:               make(map[int]Connection),
		stopSendingJoinPeportChan: make(chan int),
	}
}

func (s *service) ToggleAll(ctx context.Context, msg byte) {
	// репорт на лив из группы
	if msg == LeaveGroup {
		for _, fil := range s.db {
			if fil.IsIgmpOn {
				go s.runLeaveWorker(fil)
			}
		}
	} else {
		for _, fil := range s.db {
			if !fil.IsIgmpOn {
				go s.runJoinWorker(fil)
			}
		}
	}
}

func (s *service) runJoinWorker(f *filter.Filter) {
	// новый пул соединения
	conn, err := NewConnection(f.DstIP)
	if err != nil {
		return
	}
	s.workingPool[f.Id] = conn

	loop := time.NewTicker(2 * time.Second)
	// пакеты для Join report
	masterPacketJoin := s.newIgmpMsg(JoinReport, net.ParseIP(f.MasterIP))
	slavePacketJoin := s.newIgmpMsg(JoinReport, net.ParseIP(f.SlaveIP))

	// меняем статус, что отправка igmp включена
	f.IsIgmpOn = true

	for {
		select {
		case <-loop.C:
			go conn.Send(masterPacketJoin, net.ParseIP(f.MasterIP))
			go conn.Send(slavePacketJoin, net.ParseIP(f.SlaveIP))
		case id := <-s.stopSendingJoinPeportChan:
			if id == f.Id {
				return
			}
		}
	}
}

func (s *service) ToggleByID(ctx context.Context, id int, msg byte) error {
	f, ok := s.db[id]
	if !ok {
		return errors.New("Такого id не существует")
	}
	if f.IsIgmpOn && msg == JoinReport {
		return errors.New("IGMP уже включен для этой связки")
	}
	if msg == JoinReport {
		go s.runJoinWorker(f)
	} else {
		go s.runLeaveWorker(f)
	}
	return nil
}

func (s *service) newIgmpMsg(msgType byte, multicastAddr net.IP) []byte {
	var packet bytes.Buffer
	packet.WriteByte(msgType)
	packet.WriteByte(0x00)
	packet.Write([]byte{0x00, 0x00})
	packet.Write(multicastAddr.To4())

	checksum := s.calculateChecksum(packet.Bytes())
	binary.BigEndian.PutUint16(packet.Bytes()[2:], checksum)

	return packet.Bytes()
}

func (s *service) calculateChecksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		if i+1 < len(data) {
			sum += uint32(data[i])<<8 | uint32(data[i+1])
		} else {
			sum += uint32(data[i]) << 8
		}
	}

	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return uint16(^sum)
}

func (s *service) runLeaveWorker(f *filter.Filter) {
	conn, ok := s.workingPool[f.Id]
	if !ok {
		return
	}

	go func() {
		s.stopSendingJoinPeportChan <- f.Id
	}()

	go conn.Send(s.newIgmpMsg(LeaveGroup, net.ParseIP(f.MasterIP)), net.ParseIP(f.MasterIP))
	go conn.Send(s.newIgmpMsg(LeaveGroup, net.ParseIP(f.SlaveIP)), net.ParseIP(f.SlaveIP))

	conn.Close()
	// удаляем соединение из пула
	defer delete(s.workingPool, f.Id)
}
