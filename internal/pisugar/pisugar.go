// Package pisugar talks to pisugar-server's TCP interface (default 127.0.0.1:8423).
package pisugar

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Addr    string
	Timeout time.Duration // per command, including waiting for the server to come up after boot
}

func New(addr string) *Client { return &Client{Addr: addr, Timeout: 5 * time.Second} }

// exec sends one command and returns the value of the "<key>: <value>" reply line.
func (c *Client) exec(cmd string) (string, error) {
	fields := strings.Fields(cmd)
	key := fields[0]
	if key == "get" && len(fields) > 1 {
		key = fields[1]
	}
	deadline := time.Now().Add(c.Timeout)
	var conn net.Conn
	var err error
	for {
		conn, err = net.DialTimeout("tcp", c.Addr, time.Second)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond) // server may still be starting
	}
	if err != nil {
		return "", fmt.Errorf("pisugar: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
		return "", fmt.Errorf("pisugar %q: %w", cmd, err)
	}
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(v), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("pisugar %q: %w", cmd, err)
	}
	return "", fmt.Errorf("pisugar %q: no reply", cmd)
}

func (c *Client) action(cmd string) error {
	v, err := c.exec(cmd)
	if err != nil {
		return err
	}
	if v != "done" {
		return fmt.Errorf("pisugar %q: %s", cmd, v)
	}
	return nil
}

// Battery returns the charge level in percent.
func (c *Client) Battery() (float64, error) {
	v, err := c.exec("get battery")
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(v, 64)
}

// PowerPlugged reports whether USB power is connected (PiSugar 3).
func (c *Client) PowerPlugged() (bool, error) {
	v, err := c.exec("get battery_power_plugged")
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(v)
}

// RTCToPi sets the system clock from the PiSugar RTC.
func (c *Client) RTCToPi() error { return c.action("rtc_rtc2pi") }

// PiToRTC writes the (NTP-synchronised) system clock into the RTC.
func (c *Client) PiToRTC() error { return c.action("rtc_pi2rtc") }

// SetAlarm programs the wake-up; repeat is a weekday bitmask (127 = every day).
func (c *Client) SetAlarm(t time.Time, repeat int) error {
	return c.action(fmt.Sprintf("rtc_alarm_set %s %d", t.Truncate(time.Second).Format(time.RFC3339), repeat))
}
