package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type NATSConnectionMonitor interface {
	ListConnections(ctx context.Context) (NATSConnectionsResponse, error)
}

type NATSConnectionsResponse struct {
	ServerID       string           `json:"server_id"`
	Now            string           `json:"now"`
	Total          int              `json:"total"`
	NumConnections int              `json:"num_connections"`
	Connections    []NATSConnection `json:"connections"`
}

type NATSConnection struct {
	CID           uint64   `json:"cid"`
	Name          string   `json:"name"`
	IP            string   `json:"ip"`
	Port          int      `json:"port"`
	Start         string   `json:"start"`
	LastActivity  string   `json:"last_activity"`
	Uptime        string   `json:"uptime"`
	Idle          string   `json:"idle"`
	RTT           string   `json:"rtt"`
	InMsgs        int64    `json:"in_msgs"`
	OutMsgs       int64    `json:"out_msgs"`
	PendingBytes  int64    `json:"pending_bytes"`
	Subscriptions []string `json:"subscriptions"`
}

type LocationHealthResponse struct {
	Now       string               `json:"now"`
	Locations []LocationHealthItem `json:"locations"`
}

type LocationHealthItem struct {
	Code                string     `json:"code"`
	Name                string     `json:"name"`
	Status              string     `json:"status"`
	RackCount           int        `json:"rack_count"`
	OnlineRacks         int        `json:"online_racks"`
	DegradedRacks       int        `json:"degraded_racks"`
	OfflineRacks        int        `json:"offline_racks"`
	MaintenanceRacks    int        `json:"maintenance_racks"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at"`
	NATSConnected       bool       `json:"nats_connected"`
	NATSConnectionCount int        `json:"nats_connection_count"`
	InMsgs              int64      `json:"in_msgs"`
	OutMsgs             int64      `json:"out_msgs"`
	Subscriptions       []string   `json:"subscriptions"`
}

type HTTPNATSConnectionMonitor struct {
	baseURL string
	client  *http.Client
}

func buildLocationHealth(racks []RackHealthRack, connections NATSConnectionsResponse) LocationHealthResponse {
	locations := map[string]*LocationHealthItem{}
	for _, rack := range racks {
		code := normalizeLocationCode(firstNonEmpty(rack.LocationCode, rack.Location, rack.ID))
		name := firstNonEmpty(rack.Location, code)
		item := locations[code]
		if item == nil {
			item = &LocationHealthItem{Code: code, Name: name, Status: "offline"}
			locations[code] = item
		}
		item.RackCount++
		switch rack.Status {
		case "online":
			item.OnlineRacks++
		case "degraded":
			item.DegradedRacks++
		case "maintenance":
			item.MaintenanceRacks++
		default:
			item.OfflineRacks++
		}
		if rack.LastHeartbeatAt != nil && (item.LastHeartbeatAt == nil || rack.LastHeartbeatAt.After(*item.LastHeartbeatAt)) {
			item.LastHeartbeatAt = rack.LastHeartbeatAt
		}
	}

	for _, connection := range connections.Connections {
		for code, item := range locations {
			if !connectionMatchesLocation(connection, code) {
				continue
			}
			item.NATSConnected = true
			item.NATSConnectionCount++
			item.InMsgs += connection.InMsgs
			item.OutMsgs += connection.OutMsgs
			item.Subscriptions = appendUniqueStrings(item.Subscriptions, matchingSubscriptions(connection.Subscriptions, code)...)
		}
	}

	items := make([]LocationHealthItem, 0, len(locations))
	for _, item := range locations {
		item.Status = locationHealthStatus(item)
		sort.Strings(item.Subscriptions)
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Code < items[j].Code
	})

	return LocationHealthResponse{Now: firstNonEmpty(connections.Now, time.Now().UTC().Format(time.RFC3339Nano)), Locations: items}
}

type RackHealthRack struct {
	ID              string
	Location        string
	LocationCode    string
	Status          string
	LastHeartbeatAt *time.Time
}

func connectionMatchesLocation(connection NATSConnection, code string) bool {
	if code == "" {
		return false
	}
	if strings.Contains(strings.ToLower(connection.Name), code) {
		return true
	}
	for _, subject := range connection.Subscriptions {
		normalized := strings.ToLower(subject)
		if strings.HasPrefix(normalized, "dc."+code+".") || strings.Contains(normalized, ".dc."+code+".") {
			return true
		}
	}
	return false
}

func matchingSubscriptions(subscriptions []string, code string) []string {
	matches := []string{}
	for _, subject := range subscriptions {
		normalized := strings.ToLower(subject)
		if strings.HasPrefix(normalized, "dc."+code+".") || strings.Contains(normalized, ".dc."+code+".") {
			matches = append(matches, subject)
		}
	}
	return matches
}

func locationHealthStatus(item *LocationHealthItem) string {
	if item.MaintenanceRacks > 0 && item.OnlineRacks == 0 && item.DegradedRacks == 0 {
		return "maintenance"
	}
	if item.OnlineRacks > 0 && item.NATSConnected {
		if item.DegradedRacks > 0 || item.OfflineRacks > 0 {
			return "degraded"
		}
		return "online"
	}
	if item.OnlineRacks > 0 || item.NATSConnected {
		return "degraded"
	}
	if item.DegradedRacks > 0 {
		return "degraded"
	}
	return "offline"
}

func normalizeLocationCode(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func WithNATSConnectionMonitor(monitor NATSConnectionMonitor) HandlerOption {
	return func(h *Handler) {
		h.natsMonitor = monitor
	}
}

func NewHTTPNATSConnectionMonitor(baseURL string) *HTTPNATSConnectionMonitor {
	return &HTTPNATSConnectionMonitor{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 4 * time.Second},
	}
}

func (m *HTTPNATSConnectionMonitor) ListConnections(ctx context.Context) (NATSConnectionsResponse, error) {
	endpoint, err := url.Parse(m.baseURL + "/connz")
	if err != nil {
		return NATSConnectionsResponse{}, err
	}
	query := endpoint.Query()
	query.Set("subs", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return NATSConnectionsResponse{}, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return NATSConnectionsResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return NATSConnectionsResponse{}, fmt.Errorf("nats monitoring returned %s", resp.Status)
	}

	var payload struct {
		ServerID       string `json:"server_id"`
		Now            string `json:"now"`
		Total          int    `json:"total"`
		NumConnections int    `json:"num_connections"`
		Connections    []struct {
			CID           uint64   `json:"cid"`
			Name          string   `json:"name"`
			IP            string   `json:"ip"`
			Port          int      `json:"port"`
			Start         string   `json:"start"`
			LastActivity  string   `json:"last_activity"`
			Uptime        string   `json:"uptime"`
			Idle          string   `json:"idle"`
			RTT           string   `json:"rtt"`
			InMsgs        int64    `json:"in_msgs"`
			OutMsgs       int64    `json:"out_msgs"`
			PendingBytes  int64    `json:"pending_bytes"`
			Subscriptions []string `json:"subscriptions_list"`
		} `json:"connections"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return NATSConnectionsResponse{}, err
	}

	out := NATSConnectionsResponse{
		ServerID:       payload.ServerID,
		Now:            payload.Now,
		Total:          payload.Total,
		NumConnections: payload.NumConnections,
		Connections:    make([]NATSConnection, 0, len(payload.Connections)),
	}
	for _, conn := range payload.Connections {
		out.Connections = append(out.Connections, NATSConnection{
			CID:           conn.CID,
			Name:          conn.Name,
			IP:            conn.IP,
			Port:          conn.Port,
			Start:         conn.Start,
			LastActivity:  conn.LastActivity,
			Uptime:        conn.Uptime,
			Idle:          conn.Idle,
			RTT:           conn.RTT,
			InMsgs:        conn.InMsgs,
			OutMsgs:       conn.OutMsgs,
			PendingBytes:  conn.PendingBytes,
			Subscriptions: conn.Subscriptions,
		})
	}
	return out, nil
}
