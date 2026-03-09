package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"jarvis-gateway/internal/config"
)

type CalendarEvent struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Location  string    `json:"location"`
	HangoutLink string  `json:"hangoutLink"`
}

type CalendarScheduler struct {
	cfg           *config.Config
	sentReminders map[string]time.Time
	mu            sync.Mutex
	stopCh        chan struct{}
}

func NewCalendarScheduler(cfg *config.Config) *CalendarScheduler {
	return &CalendarScheduler{
		cfg:           cfg,
		sentReminders: make(map[string]time.Time),
		stopCh:        make(chan struct{}),
	}
}

func (s *CalendarScheduler) Start() {
	go s.run()
	log.Println("Calendar scheduler started (every 5 min)")
}

func (s *CalendarScheduler) Stop() {
	close(s.stopCh)
}

func (s *CalendarScheduler) run() {
	// Initial check after 30 seconds
	time.Sleep(30 * time.Second)
	s.checkCalendar()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkCalendar()
		case <-s.stopCh:
			return
		}
	}
}

func (s *CalendarScheduler) checkCalendar() {
	events, err := s.getUpcomingEvents()
	if err != nil {
		log.Printf("Calendar check error: %v", err)
		return
	}

	now := time.Now()
	reminderWindow := 15 * time.Minute

	for _, event := range events {
		timeUntil := event.Start.Sub(now)

		// Skip if event already started or too far away
		if timeUntil < 0 || timeUntil > reminderWindow+5*time.Minute {
			continue
		}

		// Skip if already reminded
		s.mu.Lock()
		if _, sent := s.sentReminders[event.ID]; sent {
			s.mu.Unlock()
			continue
		}
		s.sentReminders[event.ID] = now
		s.mu.Unlock()

		// Send reminder
		s.sendReminder(event, int(timeUntil.Minutes()))
	}

	// Cleanup old reminders
	s.cleanupOldReminders()
}

func (s *CalendarScheduler) getUpcomingEvents() ([]CalendarEvent, error) {
	// Use gog to get calendar events
	from := time.Now().Format(time.RFC3339)
	to := time.Now().Add(30 * time.Minute).Format(time.RFC3339)

	cmd := exec.Command("gog", "calendar", "events",
		"--from", from,
		"--to", to,
		"--max", "10",
		"-a", "danny.wasth@gmail.com",
		"-j",
	)

	// Set keyring password env
	cmd.Env = append(cmd.Environ(), "GOG_KEYRING_PASSWORD=openclaw")

	output, err := cmd.Output()
	if err != nil {
		// Fallback: try with openclaw
		return s.getEventsViaOpenClaw()
	}

	var events []CalendarEvent
	if err := json.Unmarshal(output, &events); err != nil {
		return nil, err
	}

	return events, nil
}

func (s *CalendarScheduler) getEventsViaOpenClaw() ([]CalendarEvent, error) {
	// Use openclaw to check calendar - it has gog skill configured
	cmd := exec.Command(s.cfg.OpenClawBin, "agent",
		"--to", "telegram:"+s.cfg.TelegramChatID,
		"--no-deliver",
		"-m", "Используй gog чтобы получить события календаря на ближайшие 30 минут в JSON формате. Верни только JSON массив событий без объяснений.",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Try to parse JSON from output
	outputStr := string(output)
	start := strings.Index(outputStr, "[")
	end := strings.LastIndex(outputStr, "]")

	if start == -1 || end == -1 || end <= start {
		return []CalendarEvent{}, nil
	}

	var events []CalendarEvent
	if err := json.Unmarshal([]byte(outputStr[start:end+1]), &events); err != nil {
		return []CalendarEvent{}, nil
	}

	return events, nil
}

func (s *CalendarScheduler) sendReminder(event CalendarEvent, minutesBefore int) {
	log.Printf("Sending calendar reminder: %s (in %d min)", event.Summary, minutesBefore)

	message := formatReminderMessage(event, minutesBefore)

	cmd := exec.Command(s.cfg.OpenClawBin, "agent",
		"--to", "telegram:"+s.cfg.TelegramChatID,
		"--deliver",
		"-m", message,
	)

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to send reminder: %v", err)
	}
}

func formatReminderMessage(event CalendarEvent, minutesBefore int) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Напоминание о встрече через %d минут:", minutesBefore))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**%s**", event.Summary))
	parts = append(parts, fmt.Sprintf("Время: %s - %s", event.Start.Format("15:04"), event.End.Format("15:04")))

	if event.Location != "" {
		parts = append(parts, fmt.Sprintf("Место: %s", event.Location))
	}

	if event.HangoutLink != "" {
		parts = append(parts, fmt.Sprintf("Ссылка: %s", event.HangoutLink))
	}

	return strings.Join(parts, "\n")
}

func (s *CalendarScheduler) cleanupOldReminders() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, sentAt := range s.sentReminders {
		if sentAt.Before(cutoff) {
			delete(s.sentReminders, id)
		}
	}
}
