//go:build linux

package epd

import "time"

func init() {
	open12 := func(newDriver func(Bus12, func(time.Duration)) Display) func() (Display, error) {
		return func() (Display, error) {
			bus, err := newLinuxBus12()
			if err != nil {
				return nil, err
			}
			return newDriver(bus, time.Sleep), nil
		}
	}
	openers["epd_12_in_48"] = open12(func(b Bus12, s func(time.Duration)) Display { return NewEPD12in48(b, s) })
	openers["epd_12_in_48_colour"] = open12(func(b Bus12, s func(time.Duration)) Display { return NewEPD12in48b(b, s) })
}
