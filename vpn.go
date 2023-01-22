package vpn

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"barista.run/bar"
	"barista.run/base/value"
	"barista.run/outputs"
	"barista.run/timing"
)

type Module struct {
	intf       string
	scheduler  *timing.Scheduler
	outputFunc value.Value // of func(string) bar.Output
}

func New(iface string) *Module {
	m := &Module{
		intf:      iface,
		scheduler: timing.NewScheduler(),
	}
	m.RefreshInterval(5 * time.Second)

	m.outputFunc.Set(func(in string) bar.Output {
		return outputs.Text(in)
	})

	return m
}

func (m *Module) Output(outputFunc func(State) bar.Output) *Module {
	m.outputFunc.Set(outputFunc)
	return m
}

// RefreshInterval configures the polling frequency for getloadavg.
func (m *Module) RefreshInterval(interval time.Duration) *Module {
	m.scheduler.Every(interval)
	return m
}

func (m *Module) Stream(s bar.Sink) {
	outputFunc := m.outputFunc.Get().(func(State) bar.Output)

	nextOutputFunc, done := m.outputFunc.Subscribe()
	defer done()

	data := getVpnState(m.intf)
	for {
		s.Output(outputFunc(data))

		select {
		case <-m.scheduler.C:
			data = getVpnState(m.intf)
		case <-nextOutputFunc:
			outputFunc = m.outputFunc.Get().(func(State) bar.Output)
		}
	}
}

// State represents the vpn state.
type State int

// Connected returns true if the VPN is connected.
func (s State) Connected() bool {
	return s == Connected
}

// Disconnected returns true if the VPN is off.
func (s State) Disconnected() bool {
	return s <= Disconnected
}

// Valid states for the vpn
const (
	Unknown State = -1
	Disconnected State = 0
	Waiting State = 50
	Connected State = 100
)

func getVpnState(deviceName string) State {
	execPath, err := exec.LookPath("nmcli")
	if err != nil {
		return Unknown
	}

	c := exec.Command(execPath, "d", "show", deviceName)

	o, err := c.Output()
	if err != nil {
		return Unknown
	}

	result := strings.Split(string(o), "\n")

	re := regexp.MustCompile(`^GENERAL\.STATE:\s+([0-9\.]+)\s\(.+\)$`)

	for _, l := range result {
		match := re.FindStringSubmatch(l)

		if len(match) > 0 {
			i, _ := strconv.Atoi(match[1])
			return getState(i)
		}
	}

	return Unknown
}

func getState(state int) State {
	switch state {
	case 100:
		return Connected
	case 50:
		return Waiting
	default:
		return Disconnected
	}
}
