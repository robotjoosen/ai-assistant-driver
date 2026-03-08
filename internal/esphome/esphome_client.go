package esphome

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/mycontroller-org/esphome_api/pkg/api"
	"github.com/mycontroller-org/esphome_api/pkg/connection"
	"google.golang.org/protobuf/proto"
)

const (
	msgSubscribeVoiceAssistant = 89
	msgVoiceAssistantRequest   = 90
	msgVoiceAssistantEvent     = 92
	msgVoiceAssistantAudio     = 106
)

type pendingRequest struct {
	responseTypeID uint64
	responseChan   chan proto.Message
}

type ESPHomeClient struct {
	mu       sync.Mutex
	logger   *slog.Logger
	address  string
	clientID string
	closed   bool
	wg       sync.WaitGroup
	msgChan  chan proto.Message
	stopChan chan struct{}
	conn     net.Conn
	apiConn  connection.ApiConnection

	pendingMu   sync.Mutex
	pendingReqs map[uint64]*pendingRequest
}

func NewESPHomeClient(address string, logger *slog.Logger) *ESPHomeClient {
	return &ESPHomeClient{
		address:     address,
		logger:      logger,
		msgChan:     make(chan proto.Message, 10),
		stopChan:    make(chan struct{}),
		pendingReqs: make(map[uint64]*pendingRequest),
	}
}

func (c *ESPHomeClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.apiConn != nil {
		return nil
	}

	c.logger.Info("connecting to ESPHome device", "address", c.address)

	conn, err := net.DialTimeout("tcp", c.address, 10*time.Second)
	if err != nil {
		c.logger.Error("failed to dial ESPHome device", "error", err)
		return err
	}

	c.conn = conn

	apiConn, err := connection.GetConnection(conn, 10*time.Second, "")
	if err != nil {
		c.logger.Error("failed to create API connection", "error", err)
		conn.Close()
		return err
	}

	c.apiConn = apiConn

	if err := apiConn.Handshake(); err != nil {
		c.logger.Error("handshake failed", "error", err)
		conn.Close()
		return err
	}

	c.logger.Info("connected to ESPHome device", "address", c.address)

	c.wg.Add(1)
	go c.readLoop(bufio.NewReader(conn))

	return nil
}

func (c *ESPHomeClient) readLoop(reader *bufio.Reader) {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
			msg, err := c.apiConn.Read(reader)
			if err != nil {
				c.logger.Error("read error", "error", err)
				return
			}
			c.routeMessage(msg)
		}
	}
}

func (c *ESPHomeClient) routeMessage(msg proto.Message) {
	if msg == nil {
		return
	}

	msgTypeID := api.TypeID(msg)
	c.logger.Debug("received message", "type", proto.MessageName(msg), "typeID", msgTypeID)

	c.pendingMu.Lock()
	req, found := c.pendingReqs[msgTypeID]
	c.pendingMu.Unlock()

	if found {
		select {
		case req.responseChan <- msg:
		default:
			c.logger.Warn("response channel full, dropping")
		}
		return
	}

	select {
	case c.msgChan <- msg:
	case <-c.stopChan:
	default:
		c.logger.Warn("message channel full, dropping")
	}
}

func (c *ESPHomeClient) SubscribeStates() error {
	c.logger.Debug("→ SubscribeStates")

	if err := c.apiConn.Write(&api.SubscribeStatesRequest{}); err != nil {
		c.logger.Error("SubscribeStates failed", "error", err)
		return err
	}

	c.logger.Debug("SubscribeStates sent")
	return nil
}

func (c *ESPHomeClient) Hello() error {
	c.logger.Debug("→ Hello")

	_, err := c.sendAndWaitForResponse(&api.HelloRequest{
		ClientInfo: c.clientID,
	}, 2)
	if err != nil {
		c.logger.Error("Hello failed", "error", err)
		return err
	}

	c.logger.Info("Hello successful")
	return nil
}

func (c *ESPHomeClient) Login(password string) error {
	c.logger.Debug("→ Connect (login)")

	_, err := c.sendAndWaitForResponse(&api.ConnectRequest{
		Password: password,
	}, 3)
	if err != nil {
		c.logger.Warn("Login failed (continuing anyway)", "error", err)
		return err
	}

	c.logger.Info("Login successful")
	return nil
}

func (c *ESPHomeClient) sendAndWaitForResponse(msg proto.Message, responseTypeID uint64) (proto.Message, error) {
	c.logger.Debug("→ sending message (waiting)", "type", proto.MessageName(msg), "responseTypeID", responseTypeID)

	if err := c.apiConn.Write(msg); err != nil {
		c.logger.Error("write failed", "type", proto.MessageName(msg), "error", err)
		return nil, err
	}

	responseChan := make(chan proto.Message, 1)

	c.pendingMu.Lock()
	c.pendingReqs[responseTypeID] = &pendingRequest{
		responseTypeID: responseTypeID,
		responseChan:   responseChan,
	}
	c.pendingMu.Unlock()

	select {
	case resp := <-responseChan:
		c.pendingMu.Lock()
		delete(c.pendingReqs, responseTypeID)
		c.pendingMu.Unlock()
		return resp, nil
	case <-time.After(10 * time.Second):
		c.pendingMu.Lock()
		delete(c.pendingReqs, responseTypeID)
		c.pendingMu.Unlock()
		c.logger.Error("timeout waiting for response", "type", proto.MessageName(msg), "responseTypeID", responseTypeID)
		return nil, context.DeadlineExceeded
	}
}

func (c *ESPHomeClient) sendWithTypeID(msgTypeID uint64, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		c.logger.Error("failed to marshal message", "type", proto.MessageName(msg), "error", err)
		return err
	}

	var buf bytes.Buffer
	buf.WriteByte(0x00)

	varintBuf := make([]byte, 10)
	n := binary.PutUvarint(varintBuf, uint64(len(data)))
	buf.Write(varintBuf[:n])

	n = binary.PutUvarint(varintBuf, msgTypeID)
	buf.Write(varintBuf[:n])

	buf.Write(data)

	frame := buf.Bytes()
	c.logger.Debug("→ sending raw message", "typeID", msgTypeID, "type", proto.MessageName(msg), "size", len(data), "frame", formatHex(frame))

	if _, err := c.conn.Write(frame); err != nil {
		c.logger.Error("send failed", "type", proto.MessageName(msg), "error", err)
		return err
	}

	return nil
}

func formatHex(data []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}

func (c *ESPHomeClient) SubscribeVoiceAssistant() error {
	c.logger.Debug("→ SubscribeVoiceAssistant")

	req := &api.SubscribeVoiceAssistantRequest{
		Subscribe: true,
		Flags:     uint32(api.VoiceAssistantSubscribeFlag_VOICE_ASSISTANT_SUBSCRIBE_API_AUDIO),
	}

	if err := c.sendWithTypeID(msgSubscribeVoiceAssistant, req); err != nil {
		c.logger.Error("SubscribeVoiceAssistant failed", "error", err)
		return err
	}

	c.logger.Info("subscribed to voice assistant")
	return nil
}

func (c *ESPHomeClient) StartVoiceAssistant() error {
	c.logger.Debug("→ VoiceAssistantRequest (start)")

	req := &api.VoiceAssistantRequest{
		Start: true,
		Flags: uint32(api.VoiceAssistantRequestFlag_VOICE_ASSISTANT_REQUEST_USE_WAKE_WORD),
	}

	if err := c.sendWithTypeID(msgVoiceAssistantRequest, req); err != nil {
		c.logger.Error("StartVoiceAssistant failed", "error", err)
		return err
	}

	c.logger.Info("voice assistant started")
	return nil
}

func (c *ESPHomeClient) Messages() <-chan proto.Message {
	return c.msgChan
}

func (c *ESPHomeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.stopChan)

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.apiConn != nil {
		c.apiConn = nil
	}

	c.wg.Wait()

	close(c.msgChan)

	return nil
}
