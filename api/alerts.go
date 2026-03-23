package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	orefAlertsURL = "https://www.oref.org.il/WarningMessages/alert/alerts.json"
	pollInterval  = 2 * time.Second
	dedupTTL      = 10 * time.Minute
	summaryWindow = 12 * time.Hour
)

type timedAlert struct {
	alert OrefAlert
	seenAt time.Time
}

type AlertPoller struct {
	mu         sync.Mutex
	seen       map[string]time.Time
	lastAlerts []OrefAlert
	history    []timedAlert
}

func NewAlertPoller() *AlertPoller {
	return &AlertPoller{seen: make(map[string]time.Time)}
}

func (p *AlertPoller) Start(store *SubscriberStore, broadcast func(*Subscriber, OrefAlert)) {
	pollTicker := time.NewTicker(pollInterval)
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer pollTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			alerts, err := p.fetchAlerts()
			if err != nil {
				continue
			}
			for _, alert := range alerts {
				if p.isDuplicate(alert.ID) {
					continue
				}
				p.markSeen(alert.ID)
				p.storeRecent(alert)
				for _, sub := range store.All() {
					go broadcast(sub, alert)
				}
			}
		case <-cleanupTicker.C:
			p.cleanupSeen()
		}
	}
}

func (p *AlertPoller) fetchAlerts() ([]OrefAlert, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", orefAlertsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://www.oref.org.il/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, nil
	}

	var alert OrefAlert
	if err := json.Unmarshal([]byte(trimmed), &alert); err != nil {
		return nil, err
	}
	if alert.ID == "" {
		return nil, nil
	}
	return []OrefAlert{alert}, nil
}

func (p *AlertPoller) isDuplicate(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, exists := p.seen[id]
	return exists
}

func (p *AlertPoller) markSeen(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen[id] = time.Now()
}

func (p *AlertPoller) storeRecent(alert OrefAlert) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAlerts = append([]OrefAlert{alert}, p.lastAlerts...)
	if len(p.lastAlerts) > 20 {
		p.lastAlerts = p.lastAlerts[:20]
	}
	p.history = append(p.history, timedAlert{alert: alert, seenAt: time.Now()})
}

// History returns all alerts seen in the last 12 hours.
func (p *AlertPoller) History() []OrefAlert {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-summaryWindow)
	var result []OrefAlert
	for _, ta := range p.history {
		if ta.seenAt.After(cutoff) {
			result = append(result, ta.alert)
		}
	}
	return result
}

func (p *AlertPoller) cleanupSeen() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for id, t := range p.seen {
		if now.Sub(t) > dedupTTL {
			delete(p.seen, id)
		}
	}
}

func (p *AlertPoller) RecentAlerts() []OrefAlert {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]OrefAlert, len(p.lastAlerts))
	copy(result, p.lastAlerts)
	return result
}

// broadcastAlert sends an alert to a single subscriber with location-based prioritization.
func (api *ApiManager) broadcastAlert(sub *Subscriber, alert OrefAlert) {
	var msg string
	if sub.Language == "en" {
		translated, err := translateAreas(alert.Data)
		if err == nil {
			msg = buildMessage(sub, alert, translated, true)
		} else {
			msg = buildMessage(sub, alert, alert.Data, false)
		}
	} else {
		msg = buildMessage(sub, alert, alert.Data, false)
	}
	api.sendWhatsAppMessage(sub.Phone, msg) //nolint:errcheck
}

func buildMessage(sub *Subscriber, alert OrefAlert, areas []string, inEnglish bool) string {
	areaStr := strings.Join(areas, ", ")
	prefix := "📢"
	urgent := false

	if sub.Location != "" {
		for _, area := range alert.Data {
			if strings.Contains(area, sub.Location) || strings.Contains(sub.Location, area) {
				urgent = true
				break
			}
		}
		if !urgent && inEnglish {
			for _, area := range areas {
				if strings.EqualFold(area, sub.Location) {
					urgent = true
					break
				}
			}
		}
	}

	if urgent {
		prefix = "🚨 *YOUR AREA*"
	}

	if inEnglish {
		return fmt.Sprintf("%s\n*Missile/Rocket Alert*\nAreas: %s\nEnter a protected space and remain for 10 minutes.", prefix, areaStr)
	}
	return fmt.Sprintf("%s *התראה*\n*%s*\nאזורים: %s\n%s", prefix, alert.Title, areaStr, alert.Desc)
}

func translateAreas(areas []string) ([]string, error) {
	prompt := fmt.Sprintf(
		"Translate these Israeli city/area names from Hebrew to English. Return a comma-separated list only, no extra text: %s",
		strings.Join(areas, ", "),
	)
	result, err := claudeComplete("You are a translator.", prompt)
	if err != nil {
		return nil, err
	}
	return strings.Split(result, ", "), nil
}
