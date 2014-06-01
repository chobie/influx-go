package influx

import (
	"bytes"
	"fmt"
	"net"
	"errors"

	"influx/protocol"
)

type Client struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Conn     net.Conn
	Buffer   *bytes.Buffer
}

type HandshakeState int32

const (
	HandshakeState_INITIALIZED HandshakeState = 0
	HandshakeState_WAIT_STARTUP_RESPONSE HandshakeState = 1
	HandshakeState_AUTHENTICATION_REQUEST HandshakeState = 2
	HandshakeState_WAIT_AUTHENTICATION_RESPONSE HandshakeState = 3
	HandshakeState_PROCESS_READY HandshakeState = 4
	HandshakeState_SEND_AUTHENTICATION HandshakeState = 5
	HandshakeState_FINISHED HandshakeState = 6
	HandshakeState_UPGRADE_SSL HandshakeState = 7
	HandshakeState_ERROR HandshakeState = 999
)

var Greeting_STARTUP_MESSAGE = protocol.Greeting_STARTUP_MESSAGE
var Greeting_AUTHENTICATION = protocol.Greeting_AUTHENTICATION
var Greeting_AUTHENTICATION_OK = protocol.Greeting_AUTHENTICATION_OK
var Greeting_Configuration_PLAIN = protocol.Greeting_Configuration_PLAIN
var Greeting_Authentication_CLEARTEXT_PASSWORD = protocol.Greeting_Authentication_CLEARTEXT_PASSWORD
var Greeting_MESSAGE_OPTION = protocol.Greeting_MESSAGE_OPTION
var Greeting_SSL_UPGRADE = protocol.Greeting_SSL_UPGRADE
var Greeting_Configuration_REQUIRED = protocol.Greeting_Configuration_REQUIRED
var Greeting_COMMAND_READY = protocol.Greeting_COMMAND_READY

// Commands
var COMMAND_LISTDATABASE = protocol.Command_LISTDATABASE
var COMMAND_WRITESERIES = protocol.Command_WRITESERIES
var COMMAND_QUERY = protocol.Command_QUERY
var COMMAND_CLOSE = protocol.Command_CLOSE
var COMMAND_PING = protocol.Command_PING
var COMMAND_CREATEDATABASE = protocol.Command_CREATEDATABASE
var COMMAND_DROPDATABASE = protocol.Command_DROPDATABASE

func NewTcpClient(host, port, user, password, database string) (*Client, error) {
	client := &Client{host, port, user, password, database, nil, nil}
	err := client.connect("tcp", fmt.Sprintf("%s:%s", host, port))
	return client, err
}

func NewUnixClient(host, user, password, database string) (*Client, error) {
	client := &Client{host, "0", user, password, database, nil, nil}
	err := client.connect("unix", host)
	return client, err
}

func (self *Client) connect(protocol, hostspec string) (error) {
	conn, err := net.Dial(protocol, hostspec)
	if err != nil {
		return err
	}
	self.Conn = conn

	message := make([]byte, 0, 8192)
	self.Buffer = bytes.NewBuffer(message)
	return self.handshake()
}

func (self *Client) Close() error {
	request := &protocol.Command{
		Type: &COMMAND_CLOSE,
	}

	fmt.Printf("Close request: %+v", request)
	self.WriteRequest(request)
	self.Conn.Close()
	return nil
}

func (self *Client) Query(query string) error {
	request := &protocol.Command{
		Type: &COMMAND_QUERY,
		Query: &protocol.Command_Query{
			Query: []byte(query),
		},
	}

	if err := self.WriteRequest(request); err != nil {
		return err
	}

	for {
		resp := &protocol.Command{}
		if err := self.ReadCommand(resp); err != nil {
			return err
		}
		fmt.Printf("[Query Response]: %+v\n", resp)
		if *resp.Continue == false {
			break
		}

		fmt.Printf(".")
	}

	return nil
}

func (self *Client) WriteSeries(series []*protocol.Series) error {
	if len(series) < 1 {
		return errors.New(fmt.Sprintf("at least 1 series required"))
	}

	// verify series
	for _, s := range series {
		count := len(s.GetFields())
		if count < 1 {
			return errors.New(fmt.Sprintf("at least 1 fields required"))
		}

		for index, point := range s.GetPoints() {
			cnt := len(point.GetValues())
			if cnt < 1 {
				return errors.New(fmt.Sprintf("at least 1 fields required"))
			}

			if cnt != count {
				return errors.New(fmt.Sprintf("Fields and FiledValues are missmatched. Fields specified %d but %d at %d index", count, cnt, index))
			}
		}
	}

	request := &protocol.Command{
		Type: &COMMAND_WRITESERIES,
		Series: &protocol.Command_Series{
			Series: series,
		},
	}

	if err := self.WriteRequest(request); err != nil {
		return err
	}

	resp := &protocol.Command{}
	if err := self.ReadCommand(resp); err != nil {
		return err
	}

	return nil
}

func (self *Client) ListDatabase() ([]string, error) {
	request := &protocol.Command{
		Type: &COMMAND_LISTDATABASE,
	}
	if err := self.WriteRequest(request); err != nil {
		return nil, err
	}

	response := &protocol.Command{}
	if err := self.ReadCommand(response); err != nil {
		return nil, err
	}

	fmt.Printf("[List Database Response]: %+v\n", *response)
	return response.GetDatabase().GetName(), nil
}

func (self *Client) Ping() (bool, error) {
	request := &protocol.Command{
		Type: &COMMAND_PING,
	}
	if err := self.WriteRequest(request); err != nil {
		return false, err
	}

	response := &protocol.Command{}
	if err := self.ReadCommand(response); err != nil {
		return false, err
	}

	fmt.Printf("[PING Response]: %+v\n", *response)
	return true, nil
}

func (self *Client) CreateDatabase(name string) (bool, error) {
	request := &protocol.Command{
		Type: &COMMAND_CREATEDATABASE,
		Database: &protocol.Command_Database{
			Name: []string{name},
		},
	}

	if err := self.WriteRequest(request); err != nil {
		return false, err
	}

	response := &protocol.Command{}
	if err := self.ReadCommand(response); err != nil {
		return false, err
	}

	fmt.Printf("[Create Database Response]: %+v\n", *response)
	return true, nil
}

func (self *Client) DropDatabase(name string) (bool, error) {
	request := &protocol.Command{
		Type: &COMMAND_DROPDATABASE,
		Database: &protocol.Command_Database{
			Name: []string{name},
		},
	}

	if err := self.WriteRequest(request); err != nil {
		return false, err
	}

	response := &protocol.Command{}
	if err := self.ReadCommand(response); err != nil {
		return false, err
	}

	fmt.Printf("[DROP Database Response]: %+v\n", *response)
	return true, nil
}

