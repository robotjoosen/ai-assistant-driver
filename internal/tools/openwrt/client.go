package openwrt

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Client struct {
	baseURL  string
	username string
	password string
	token    string
	client   *http.Client
}

type Device struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname,omitempty"`
	Interface string `json:"interface,omitempty"`
}

type NetworkStatus struct {
	Interfaces []InterfaceStatus `json:"interfaces"`
	Traffic    []TrafficStats    `json:"traffic"`
	WiFi       []WiFiStatus      `json:"wifi"`
	DHCPLeases []DHCPLease       `json:"dhcp_leases"`
}

type InterfaceStatus struct {
	Name    string `json:"name"`
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	Up      bool   `json:"up"`
	MTU     int    `json:"mtu,omitempty"`
	MAC     string `json:"mac,omitempty"`
}

type TrafficStats struct {
	Interface string `json:"interface"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
}

type WiFiStatus struct {
	SSID     string        `json:"ssid,omitempty"`
	Channel  int           `json:"channel"`
	Mode     string        `json:"mode"`
	Signal   int           `json:"signal"`
	Clients  int           `json:"clients"`
	Device   string        `json:"device"`
	Stations []WiFiStation `json:"stations,omitempty"`
}

type WiFiStation struct {
	MAC      string `json:"mac"`
	Signal   int    `json:"signal"`
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

type DHCPLease struct {
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Hostname string `json:"hostname,omitempty"`
	Expires  string `json:"expires"`
}

func NewClient(host string, port int, username, password string) *Client {
	if host == "" {
		host = detectDefaultGateway()
	}

	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func detectDefaultGateway() string {
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		slog.Warn("failed to detect default gateway", "error", err)
		return "192.168.1.1"
	}

	parts := strings.Fields(string(output))
	for i, part := range parts {
		if part == "via" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return "192.168.1.1"
}

func (c *Client) login() error {
	authURL := c.baseURL + "/cgi-bin/luci/rpc/auth?method=login"

	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	reqBody, err := json.Marshal(loginRequest{
		Username: c.username,
		Password: c.password,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest("POST", authURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to login to router: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	var token string
	if err := json.Unmarshal(body, &token); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	c.token = token
	return nil
}

func (c *Client) getDevices() ([]Device, error) {
	if c.token == "" {
		if err := c.login(); err != nil {
			return nil, err
		}
	}

	dhcpURL := c.baseURL + "/cgi-bin/luci/rpc/uci?auth=" + c.token + "&method=get&params=[" + `"network"` + "]"

	req, err := http.NewRequest("GET", dhcpURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	devices, err := c.arpLookup()
	if err != nil {
		slog.Warn("ARP lookup failed", "error", err)
	}

	if len(devices) == 0 {
		slog.Info("no devices found via LuCi, trying ARP table")
		return c.arpTableLookup()
	}

	return devices, nil
}

func (c *Client) arpLookup() ([]Device, error) {
	cmd := exec.Command("arp", "-a")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run arp command: %w", err)
	}

	return parseARP(strings.NewReader(string(output))), nil
}

func (c *Client) arpTableLookup() ([]Device, error) {
	arpFile, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/net/arp: %w", err)
	}
	defer arpFile.Close()

	return parseARP(arpFile), nil
}

func parseARP(r io.Reader) []Device {
	var devices []Device

	data, _ := io.ReadAll(r)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return devices
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		ip := fields[0]
		mac := fields[3]

		if mac == "00:00:00:00:00:00" || mac == "" {
			continue
		}

		hostname, _ := net.LookupAddr(ip)

		devices = append(devices, Device{
			IP:       ip,
			MAC:      mac,
			Hostname: strings.TrimSuffix(hostname[0], "."),
		})
	}

	return devices
}

func (c *Client) GetConnectedClients() ([]Device, error) {
	if c.token == "" {
		if err := c.login(); err != nil {
			slog.Warn("LuCi login failed, falling back to ARP", "error", err)
			return c.arpTableLookup()
		}
	}

	return c.getDevices()
}

func (c *Client) GetNetworkStatus() (*NetworkStatus, error) {
	if c.token == "" {
		if err := c.login(); err != nil {
			slog.Warn("LuCi login failed, falling back to local methods", "error", err)
			return c.getLocalNetworkStatus(), nil
		}
	}

	status := &NetworkStatus{
		Interfaces: c.getLuCiInterfaces(),
		Traffic:    c.getTrafficStats(),
		WiFi:       c.getLuCiWiFiStatus(),
		DHCPLeases: c.getLuCiDHCPLeases(),
	}

	if len(status.Interfaces) == 0 {
		status.Interfaces = c.getLocalInterfaces()
	}

	if len(status.WiFi) == 0 {
		status.WiFi = c.getLocalWiFiStatus()
	}

	if len(status.DHCPLeases) == 0 {
		status.DHCPLeases = c.getLocalDHCPLeases()
	}

	return status, nil
}

func (c *Client) getLuCiInterfaces() []InterfaceStatus {
	interfaces := []InterfaceStatus{}

	networkURL := c.baseURL + "/cgi-bin/luci/rpc/uci?auth=" + c.token + "&method=get_all&params=[" + `"network"` + "]"
	respBody, err := c.makeRequest(networkURL)
	if err != nil {
		slog.Debug("failed to get LuCi interfaces", "error", err)
		return interfaces
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return interfaces
	}

	network, ok := result["result"].(map[string]interface{})
	if !ok {
		return interfaces
	}

	for name, value := range network {
		if !strings.HasPrefix(name, "interface_") {
			continue
		}

		ifaceMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		iface := InterfaceStatus{
			Name: strings.TrimPrefix(name, "interface_"),
			Up:   ifaceMap[".type"] != nil,
		}

		if ip, ok := ifaceMap["ipaddr"].(string); ok {
			iface.IP = ip
		}
		if mask, ok := ifaceMap["netmask"].(string); ok {
			iface.Netmask = mask
		}
		if gateway, ok := ifaceMap["gateway"].(string); ok {
			iface.Gateway = gateway
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces
}

func (c *Client) getTrafficStats() []TrafficStats {
	var stats []TrafficStats

	procNetDev, err := os.Open("/proc/net/dev")
	if err != nil {
		slog.Debug("failed to open /proc/net/dev", "error", err)
		return stats
	}
	defer procNetDev.Close()

	data, _ := io.ReadAll(procNetDev)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines[2:] {
		fields := strings.Fields(line)
		if len(fields) < 17 {
			continue
		}

		name := strings.TrimSuffix(fields[0], ":")

		if name == "lo" || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "docker") {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[2], 10, 64)
		rxErrors, _ := strconv.ParseUint(fields[3], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[9], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[10], 10, 64)
		txErrors, _ := strconv.ParseUint(fields[11], 10, 64)

		stats = append(stats, TrafficStats{
			Interface: name,
			RxBytes:   rxBytes,
			TxBytes:   txBytes,
			RxPackets: rxPackets,
			TxPackets: txPackets,
			RxErrors:  rxErrors,
			TxErrors:  txErrors,
		})
	}

	return stats
}

func (c *Client) getLuCiWiFiStatus() []WiFiStatus {
	wifiURL := c.baseURL + "/cgi-bin/luci/rpc/uci?auth=" + c.token + "&method=get_all&params=[" + `"wireless"` + "]"
	respBody, err := c.makeRequest(wifiURL)
	if err != nil {
		slog.Debug("failed to get LuCi WiFi", "error", err)
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil
	}

	var wifiStatus []WiFiStatus

	wireless, ok := result["result"].(map[string]interface{})
	if !ok {
		return wifiStatus
	}

	for name, value := range wireless {
		if !strings.HasPrefix(name, "wifi-iface_") {
			continue
		}

		ifaceMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		wifi := WiFiStatus{
			Device: ifaceMap["device"].(string),
		}

		if ssid, ok := ifaceMap["ssid"].(string); ok {
			wifi.SSID = ssid
		}
		if mode, ok := ifaceMap["mode"].(string); ok {
			wifi.Mode = mode
		}

		wifiStatus = append(wifiStatus, wifi)
	}

	return wifiStatus
}

func (c *Client) getLuCiDHCPLeases() []DHCPLease {
	dhcpURL := c.baseURL + "/cgi-bin/luci/rpc/uci?auth=" + c.token + "&method=get_all&params=[" + `"dhcp"` + "," + `"@leanase"` + "]"
	respBody, err := c.makeRequest(dhcpURL)
	if err != nil {
		slog.Debug("failed to get LuCi DHCP", "error", err)
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil
	}

	var leases []DHCPLease

	dhcp, ok := result["result"].(map[string]interface{})
	if !ok {
		return leases
	}

	for name, value := range dhcp {
		if !strings.HasPrefix(name, "host_") {
			continue
		}

		hostMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		lease := DHCPLease{}

		if ip, ok := hostMap["ip"].(string); ok {
			lease.IP = ip
		}
		if mac, ok := hostMap["mac"].(string); ok {
			lease.MAC = mac
		}
		if hostname, ok := hostMap["name"].(string); ok {
			lease.Hostname = hostname
		}

		leases = append(leases, lease)
	}

	return leases
}

func (c *Client) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) getLocalNetworkStatus() *NetworkStatus {
	return &NetworkStatus{
		Interfaces: c.getLocalInterfaces(),
		Traffic:    c.getTrafficStats(),
		WiFi:       c.getLocalWiFiStatus(),
		DHCPLeases: c.getLocalDHCPLeases(),
	}
}

func (c *Client) getLocalInterfaces() []InterfaceStatus {
	cmd := exec.Command("ip", "-j", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var ifaces []struct {
		Name    string `json:"iface"`
		Address string `json:"address"`
		Prefix  int    `json:"prefixlen"`
		State   string `json:"operstate"`
	}

	if err := json.Unmarshal(output, &ifaces); err != nil {
		return nil
	}

	var result []InterfaceStatus
	for _, i := range ifaces {
		if i.Name == "lo" || strings.HasPrefix(i.Name, "br-") {
			continue
		}

		mask := c.prefixToNetmask(i.Prefix)

		result = append(result, InterfaceStatus{
			Name:    i.Name,
			IP:      i.Address,
			Netmask: mask,
			Up:      i.State == "UP",
		})
	}

	return result
}

func (c *Client) getLocalWiFiStatus() []WiFiStatus {
	cmd := exec.Command("iw", "dev")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var wifiStatus []WiFiStatus

	lines := strings.Split(string(output), "\n")
	var currentDevice string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Interface ") {
			currentDevice = strings.TrimSpace(strings.TrimPrefix(line, "Interface "))
		}
		if strings.HasPrefix(line, "ssid ") {
			ssid := strings.TrimSpace(strings.TrimPrefix(line, "ssid "))
			wifiStatus = append(wifiStatus, WiFiStatus{
				SSID:    ssid,
				Device:  currentDevice,
				Mode:    "ap",
				Channel: 0,
			})
		}
	}

	return wifiStatus
}

func (c *Client) getLocalDHCPLeases() []DHCPLease {
	leaseFiles := []string{"/tmp/dhcp.leases", "/var/lib/dhcp/dhcpd.leases"}

	var leases []DHCPLease

	for _, leaseFile := range leaseFiles {
		data, err := os.ReadFile(leaseFile)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			expires := fields[0]
			mac := fields[1]
			ip := fields[2]
			hostname := fields[3]

			leases = append(leases, DHCPLease{
				IP:       ip,
				MAC:      mac,
				Hostname: hostname,
				Expires:  expires,
			})
		}
	}

	return leases
}

func (c *Client) prefixToNetmask(prefix int) string {
	mask := net.CIDRMask(prefix, 32)
	ip := net.IP(mask).String()
	return ip
}
