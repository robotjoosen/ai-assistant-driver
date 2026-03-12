package wyoming

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type Client struct {
	conn net.Conn
}

func NewClient(host string, port int) (*Client, error) {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	log.Printf("[WYOMING] Connecting to %s", address)

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Wyoming service: %w", err)
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	log.Printf("[WYOMING] Connected successfully")

	return &Client{
		conn: conn,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

type jsonEvent struct {
	Type          string          `json:"type"`
	Data          json.RawMessage `json:"data,omitempty"`
	DataLength    int             `json:"data_length"`
	PayloadLength int             `json:"payload_length"`
}

func (c *Client) WriteEvent(event *Event, payload []byte) error {
	eventData, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	payloadLen := len(payload)
	dataLen := len(eventData)

	log.Printf("[WYOMING] Writing event: type=%s, data_len=%d, payload_len=%d",
		event.Type, dataLen, payloadLen)
	log.Printf("[WYOMING] Event data JSON: %s", string(eventData))

	jsonEvt := jsonEvent{
		Type:          string(event.Type),
		DataLength:    dataLen,
		PayloadLength: payloadLen,
	}

	jsonLine, err := json.Marshal(jsonEvt)
	if err != nil {
		return fmt.Errorf("failed to marshal json event: %w", err)
	}

	log.Printf("[WYOMING] JSON line: %s", string(jsonLine))

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	if _, err := c.conn.Write(jsonLine); err != nil {
		return fmt.Errorf("failed to write json line: %w", err)
	}

	if _, err := c.conn.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if dataLen > 0 {
		log.Printf("[WYOMING] Writing %d bytes of event data", dataLen)
		if _, err := c.conn.Write(eventData); err != nil {
			return fmt.Errorf("failed to write event data: %w", err)
		}
	}

	if payloadLen > 0 {
		log.Printf("[WYOMING] Writing %d bytes of payload", payloadLen)
		if _, err := c.conn.Write(payload); err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
	}

	c.conn.SetReadDeadline(time.Time{})

	log.Printf("[WYOMING] Event written successfully: type=%s", event.Type)

	return nil
}

func (c *Client) ReadEvent() (*Event, []byte, error) {
	log.Printf("[WYOMING] Waiting for event...")

	line, err := readLine(c.conn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read json line: %w", err)
	}

	var jsonEvt jsonEvent
	if err := json.Unmarshal(line, &jsonEvt); err != nil {
		return nil, nil, fmt.Errorf("failed to parse json event: %w", err)
	}

	log.Printf("[WYOMING] Header received: type=%s, data_len=%d, payload_len=%d",
		jsonEvt.Type, jsonEvt.DataLength, jsonEvt.PayloadLength)

	event := &Event{
		Type: EventType(jsonEvt.Type),
	}

	if jsonEvt.DataLength > 0 {
		dataBytes := make([]byte, jsonEvt.DataLength)
		if _, err := io.ReadFull(c.conn, dataBytes); err != nil {
			return nil, nil, fmt.Errorf("failed to read event data: %w", err)
		}
		log.Printf("[WYOMING] Read %d bytes of event data: %s", jsonEvt.DataLength, string(dataBytes))
		if err := json.Unmarshal(dataBytes, &event.Data); err != nil {
			return nil, nil, fmt.Errorf("failed to parse event data: %w", err)
		}
	}

	var payload []byte
	if jsonEvt.PayloadLength > 0 {
		payload = make([]byte, jsonEvt.PayloadLength)
		if _, err := io.ReadFull(c.conn, payload); err != nil {
			return nil, nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	log.Printf("[WYOMING] Event received: type=%s", event.Type)

	return event, payload, nil
}

func (c *Client) ReadEventWithTimeout(timeout time.Duration) (*Event, []byte, error) {
	log.Printf("[WYOMING] Setting read deadline: %v", timeout)
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	log.Printf("[WYOMING] Read deadline set, calling ReadEvent...")
	return c.ReadEvent()
}

func readLine(conn net.Conn) ([]byte, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		log.Printf("[WYOMING] Attempting to read from connection...")
		n, err := conn.Read(buf)
		log.Printf("[WYOMING] Read returned: n=%d, err=%v", n, err)
		if err != nil {
			return nil, err
		}
		if buf[0] == '\n' {
			break
		}
		line = append(line, buf[0])
	}
	return line, nil
}
