//go:build linux

package internal

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type Timer struct {
	fd     int
	poller *Poller
	pd     PollData
}

func NewTimer(poller *Poller) (*Timer, error) {
	fd, err := unix.TimerfdCreate(unix.CLOCK_REALTIME, unix.TFD_NONBLOCK)
	if err != nil {
		return nil, os.NewSyscallError("timerfd_create", err)
	}

	t := &Timer{
		fd:     fd,
		poller: poller,
	}
	t.pd.Fd = t.fd
	return t, nil
}

func (t *Timer) Arm(dur time.Duration, onFire func()) error {
	// first, make sure there's not another timer setup on the same fd
	if err := t.Disarm(); err != nil {
		return err
	}

	timespec := unix.NsecToTimespec(dur.Nanoseconds())
	err := unix.TimerfdSettime(t.fd, 0, &unix.ItimerSpec{
		Interval: timespec,
		Value:    timespec,
	}, nil)
	if err == nil {
		t.pd.Set(ReadEvent, func(_ error) { onFire() })
		err = t.poller.SetRead(t.fd, &t.pd)
	}

	return err
}

func (t *Timer) Disarm() error {
	if t.pd.Flags&ReadFlags != ReadFlags {
		return nil
	}
	err := unix.TimerfdSettime(t.fd, 0, &unix.ItimerSpec{}, nil)
	if err == nil {
		err = t.poller.Del(t.fd, &t.pd)
	}
	return err
}

func (t *Timer) Close() error {
	t.Disarm()
	return syscall.Close(t.fd)
}
