package igmp

type Service interface {
	Send(msg any) error
}

type service struct {
}

func NewService() Service {
	return &service{}
}

func (s service) Send(msg any) error {
	panic("")
}
