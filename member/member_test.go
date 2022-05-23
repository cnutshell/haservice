package member

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestMember(t *testing.T) {
	m, m0Handler := setupMember(t, nil)
	m, m1Handler := setupMember(t, m)
	m, _ = setupMember(t, m)

	require.Eventually(t, func() bool {
		return 2 == m0Handler.JoinNum() &&
			3 == len(m[0].Members()) &&
			0 == m0Handler.LeaveNum() &&
			0 == m1Handler.LeaveNum()
	}, 3*time.Second, 250*time.Millisecond)

	require.NoError(t, m[2].Leave())

	require.Eventually(t, func() bool {
		return 2 == m0Handler.JoinNum() &&
			3 == len(m[0].Members()) &&
			serf.StatusLeft == m[0].Members()[2].Status &&
			1 == m0Handler.LeaveNum() &&
			1 == m1Handler.LeaveNum()
	}, 3*time.Second, 250*time.Millisecond)

	// assert number of label
	require.Equal(t, 1, len(m[0].GetLables()))

	tags := map[string]string{"hello": "world"}

	// 1. SetTags with updated labels
	{
		err := m[0].Serf.SetTags(tags)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			// for m[1]: new update event
			return int32(1) == m1Handler.UpdateNum()
		}, 3*time.Second, 250*time.Millisecond)
	}

	// 2. SetTags with the same labels
	{
		err := m[0].Serf.SetTags(tags)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			// for m[1]: no update event
			return int32(1) == m1Handler.UpdateNum()
		}, 3*time.Second, 250*time.Millisecond)
	}

	// 3. SetTags with nil
	{
		err := m[0].Serf.SetTags(nil)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			// for m[0]: new update event
			return int32(2) == m1Handler.UpdateNum()
		}, 3*time.Second, 250*time.Millisecond)

		// Tags always non-nil, even invoke SetTags with nil
		require.NotNil(t, m[0].GetLables())
	}
}

func setupMember(t *testing.T, members []*Member) (
	[]*Member, *mockHandler,
) {
	id := len(members)
	port := 10000 + id
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", port)
	c := Config{
		NodeName: fmt.Sprintf("%d", id),
		BindAddr: addr,
		RpcAddr:  addr,
	}
	h := &mockHandler{}
	if len(members) > 0 {
		c.JoinAddrs = []string{
			members[0].BindAddr,
		}
	}
	m, err := NewMember(c, h, nil)
	require.NoError(t, err)
	members = append(members, m)
	return members, h
}

type mockHandler struct {
	numJoin   int32
	numLeave  int32
	numUpdate int32
}

func (h *mockHandler) OnJoin(id, addr string, tags map[string]string) error {
	atomic.AddInt32(&h.numJoin, 1)
	return nil
}

func (h *mockHandler) OnLeave(member serf.Member) error {
	atomic.AddInt32(&h.numLeave, 1)
	return nil
}

func (h *mockHandler) OnUpdate(member serf.Member) error {
	atomic.AddInt32(&h.numUpdate, 1)
	return nil
}

func (h *mockHandler) JoinNum() int32 {
	return atomic.LoadInt32(&h.numJoin)
}

func (h *mockHandler) UpdateNum() int32 {
	return atomic.LoadInt32(&h.numUpdate)
}

func (h *mockHandler) LeaveNum() int32 {
	return atomic.LoadInt32(&h.numLeave)
}
