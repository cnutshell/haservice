package member

import (
	"net"

	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
)

type EventHandler interface {
	OnJoin(serf.Member) error
	OnLeave(serf.Member) error
	OnUpdate(serf.Member) error
}

type Config struct {
	NodeName  string
	BindAddr  string
	Tags      map[string]string
	JoinAddrs []string
}

type Member struct {
	Config
	handler EventHandler
	Serf    *serf.Serf // TODO: make Serf private when API was stable
	eventCh chan serf.Event
	logger  *zap.Logger
}

func NewMember(config Config, handler EventHandler, logger *zap.Logger) (*Member, error) {
	if logger == nil {
		logger = zap.L().Named("member")
	}

	c := &Member{
		handler: handler,
		Config:  config,
		logger:  logger,
		eventCh: make(chan serf.Event),
	}

	err := c.setupSerf()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (m *Member) Members() []serf.Member {
	return m.Serf.Members()
}

func (m *Member) Leave() error {
	return m.Serf.Leave()
}

func (m *Member) GetLables() map[string]string {
	return m.Serf.LocalMember().Tags
}

func (m *Member) UpdateLabel(label, value string) error {
	tags := m.GetLables()
	if tags == nil {
		tags = make(map[string]string)
	}

	v, ok := tags[label]
	if ok && v == value {
		// no need to update
		return nil
	}

	tags[label] = value
	return m.Serf.SetTags(tags)
}

func (m *Member) setupSerf() error {
	addr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
	if err != nil {
		return err
	}

	config := serf.DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = addr.IP.String()
	config.MemberlistConfig.BindPort = addr.Port
	config.EventCh = m.eventCh
	config.Tags = m.Tags
	config.NodeName = m.Config.NodeName

	m.Serf, err = serf.Create(config)
	if err != nil {
		return err
	}

	go m.eventHandler()

	if m.JoinAddrs != nil {
		_, err = m.Serf.Join(m.JoinAddrs, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Member) eventHandler() {
	for e := range m.eventCh {
		switch e.EventType() {
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					return
				}
				m.handleLeave(member)
			}
		case serf.EventMemberJoin:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					continue
				}
				m.handleJoin(member)
			}
		case serf.EventMemberUpdate:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					continue
				}
				m.handleUpdate(member)
			}
		case serf.EventMemberReap:
		case serf.EventUser:
		case serf.EventQuery:
		default:
			panic("unknown serf event type")
		}
	}
}

func (m *Member) handleUpdate(member serf.Member) {
	if err := m.handler.OnUpdate(member); err != nil {
		m.logError(err, "fail to update", member.Name)
	}
}

func (m *Member) handleJoin(member serf.Member) {
	if err := m.handler.OnJoin(member); err != nil {
		m.logError(err, "fail to join", member.Name)
	}
}

func (m *Member) handleLeave(member serf.Member) {
	if err := m.handler.OnLeave(member); err != nil {
		m.logError(err, "fail to leave", member.Name)
	}
}

func (m *Member) isLocal(member serf.Member) bool {
	return m.Serf.LocalMember().Name == member.Name
}

func (m *Member) logError(err error, msg string, name string) {
	m.logger.Error(
		msg,
		zap.Error(err),
		zap.String("name", name),
	)
}
