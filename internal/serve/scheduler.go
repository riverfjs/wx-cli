package serve

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Schedule struct {
	ID       string `json:"id"`
	Profile  string `json:"profile"`
	Cron     string `json:"cron"`
	Command  string `json:"command"`
	Enabled  bool   `json:"enabled"`
	Created  string `json:"created"`
	LastRun  string `json:"last_run,omitempty"`
}

type Scheduler struct {
	schedules []Schedule
	mu        sync.RWMutex
}

var sched = &Scheduler{}

func schedulesPath() string {
	return filepath.Join(serveDir(), "schedules.json")
}

func (s *Scheduler) Load() {
	data, err := os.ReadFile(schedulesPath())
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	json.Unmarshal(data, &s.schedules)
	log.Printf("[scheduler] loaded %d schedules", len(s.schedules))
}

func (s *Scheduler) Save() {
	s.mu.RLock()
	data, _ := json.MarshalIndent(s.schedules, "", "  ")
	s.mu.RUnlock()
	os.MkdirAll(serveDir(), 0755)
	os.WriteFile(schedulesPath(), data, 0600)
}

func (s *Scheduler) Add(profile, cron, command string) (Schedule, error) {
	if err := validateCron(cron); err != nil {
		return Schedule{}, err
	}
	b := make([]byte, 4)
	rand.Read(b)
	sch := Schedule{
		ID:      "s" + hex.EncodeToString(b),
		Profile: profile,
		Cron:    cron,
		Command: command,
		Enabled: true,
		Created: time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"),
	}
	s.mu.Lock()
	s.schedules = append(s.schedules, sch)
	s.mu.Unlock()
	s.Save()
	return sch, nil
}

func (s *Scheduler) Del(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sch := range s.schedules {
		if sch.ID == id {
			s.schedules = append(s.schedules[:i], s.schedules[i+1:]...)
			s.Save()
			return true
		}
	}
	return false
}

func (s *Scheduler) SetEnabled(id string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sch := range s.schedules {
		if sch.ID == id {
			s.schedules[i].Enabled = enabled
			s.Save()
			return true
		}
	}
	return false
}

func (s *Scheduler) List(profile string) []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Schedule
	for _, sch := range s.schedules {
		if profile == "" || sch.Profile == profile {
			out = append(out, sch)
		}
	}
	return out
}

func (s *Scheduler) Get(id string) (Schedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sch := range s.schedules {
		if sch.ID == id {
			return sch, true
		}
	}
	return Schedule{}, false
}

func (s *Scheduler) Run(cfg *Config) {
	s.Load()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for t := range ticker.C {
		s.mu.RLock()
		var due []Schedule
		for _, sch := range s.schedules {
			if sch.Enabled && matchCron(sch.Cron, t) {
				due = append(due, sch)
			}
		}
		s.mu.RUnlock()

		for _, sch := range due {
			go s.execute(cfg, sch)
		}
	}
}

func (s *Scheduler) execute(cfg *Config, sch Schedule) {
	log.Printf("[scheduler] executing %s for %s: %s", sch.ID, sch.Profile, sch.Command)

	token := registerPushToken(sch.Profile, 10*time.Minute)

	cmd := exec.Command("bash", "-c", sch.Command)
	cmd.Env = append(os.Environ(),
		"WX_PROFILE="+sch.Profile,
		"PUSH_KEY="+token,
	)

	logPath := filepath.Join(stateDir(), "logs", "schedule_"+sch.ID+".log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		fmt.Fprintf(f, "\n=== %s ===\n", time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"))
		cmd.Stdout = f
		cmd.Stderr = f
	}

	cmd.Run()

	// update last_run
	s.mu.Lock()
	for i, sc := range s.schedules {
		if sc.ID == sch.ID {
			s.schedules[i].LastRun = time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
			break
		}
	}
	s.mu.Unlock()
	s.Save()
}

// ── cron matching ──

func validateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("cron 需要 5 个字段: 分 时 日 月 周")
	}
	return nil
}

func matchCron(expr string, t time.Time) bool {
	cst := t.In(time.FixedZone("CST", 8*3600))
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	return matchField(fields[0], cst.Minute()) &&
		matchField(fields[1], cst.Hour()) &&
		matchField(fields[2], cst.Day()) &&
		matchField(fields[3], int(cst.Month())) &&
		matchField(fields[4], int(cst.Weekday()))
}

func matchField(field string, val int) bool {
	if field == "*" {
		return true
	}
	for _, part := range strings.Split(field, ",") {
		if strings.Contains(part, "/") {
			// */5, 0/10
			sp := strings.SplitN(part, "/", 2)
			step, err := strconv.Atoi(sp[1])
			if err != nil || step <= 0 {
				continue
			}
			base := 0
			if sp[0] != "*" {
				base, _ = strconv.Atoi(sp[0])
			}
			if (val-base)%step == 0 && val >= base {
				return true
			}
		} else if strings.Contains(part, "-") {
			// 1-5
			sp := strings.SplitN(part, "-", 2)
			lo, _ := strconv.Atoi(sp[0])
			hi, _ := strconv.Atoi(sp[1])
			if val >= lo && val <= hi {
				return true
			}
		} else {
			// exact value
			n, _ := strconv.Atoi(part)
			if n == val {
				return true
			}
		}
	}
	return false
}
