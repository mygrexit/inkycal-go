package pisugar

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer answers like pisugar-server; it records received commands.
type fakeServer struct {
	ln      net.Listener
	replies map[string]string // command prefix → reply lines
	mu      sync.Mutex
	got     []string
}

func startFake(t *testing.T, replies map[string]string) *fakeServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{ln: ln, replies: replies}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				line, _ := bufio.NewReader(conn).ReadString('\n')
				line = strings.TrimSpace(line)
				s.mu.Lock()
				s.got = append(s.got, line)
				s.mu.Unlock()
				best := ""
				for prefix := range s.replies { // longest matching prefix wins
					if strings.HasPrefix(line, prefix) && len(prefix) > len(best) {
						best = prefix
					}
				}
				if best != "" {
					conn.Write([]byte(s.replies[best]))
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return s
}

func client(s *fakeServer) *Client {
	c := New(s.ln.Addr().String())
	c.Timeout = time.Second
	return c
}

func TestBatteryAndPlugged(t *testing.T) {
	s := startFake(t, map[string]string{
		"get battery_power_plugged": "battery_power_plugged: true\n",
		"get battery":               "single\nbattery: 87.5\n", // unrelated event line first
	})
	c := client(s)
	if v, err := c.Battery(); err != nil || v != 87.5 {
		t.Fatalf("battery %v %v", v, err)
	}
	if v, err := c.PowerPlugged(); err != nil || !v {
		t.Fatalf("plugged %v %v", v, err)
	}
}

func TestSetAlarmFormat(t *testing.T) {
	s := startFake(t, map[string]string{"rtc_alarm_set": "rtc_alarm_set: done\n"})
	loc := time.FixedZone("CEST", 2*3600)
	if err := client(s).SetAlarm(time.Date(2026, 10, 10, 14, 30, 0, 123, loc), 127); err != nil {
		t.Fatal(err)
	}
	if s.got[0] != "rtc_alarm_set 2026-10-10T14:30:00+02:00 127" {
		t.Errorf("sent %q", s.got[0])
	}
}

func TestActionFailure(t *testing.T) {
	s := startFake(t, map[string]string{"rtc_pi2rtc": "rtc_pi2rtc: failed\n"})
	if err := client(s).PiToRTC(); err == nil {
		t.Fatal("expected error")
	}
}

func TestServerDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	c := New(addr)
	c.Timeout = 300 * time.Millisecond
	if _, err := c.Battery(); err == nil {
		t.Fatal("expected error")
	}
}
