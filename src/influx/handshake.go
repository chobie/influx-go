package influx

import (
	"errors"
	"fmt"

	"influx/protocol"
	"crypto/tls"
)


func (self *Client) sendStartup() error {
	req := &protocol.Greeting{
		Type: &Greeting_STARTUP_MESSAGE,
		Agent: []byte("influx-go"),
		Authentication: &protocol.Greeting_Authentication{
			Name: []byte(self.User),
			Database: []byte(self.Database),
		},
		Config: &protocol.Greeting_Configuration{
			CompressType: &Greeting_Configuration_PLAIN,
		},
	}
	fmt.Printf("[Initial Request]: %+v\n", req)
	err := self.WriteRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (self *Client) processWaitStartupResponse() error {
	response := &protocol.Greeting{}
	self.readMessage(response)
	fmt.Printf("[Initial Response From Server]: %+v\n", response)

	if response.GetType() != protocol.Greeting_STARTUP_RESPONSE {
		return errors.New("Illegal sequence")
	}
	if response.GetAuthentication().GetMethod() != Greeting_Authentication_CLEARTEXT_PASSWORD {
		return errors.New("not supported")
	}
	return nil
}

func (self *Client) handshake() error {
	state := HandshakeState_INITIALIZED

	fmt.Printf("==== Begin Handshake ====\n")
	for {
		switch (state) {
		case HandshakeState_INITIALIZED:
			if e := self.sendStartup(); e != nil {
				state = HandshakeState_ERROR
				continue
			}
			state = HandshakeState_WAIT_STARTUP_RESPONSE
			break
		case HandshakeState_WAIT_STARTUP_RESPONSE:
			// 認証方式、SSLとか考える
			ack := &protocol.Greeting{}
			self.ReadGreeting(ack)
			fmt.Printf("[Response]: %+v\n", ack)
			if ack.GetConfig().GetSsl() == Greeting_Configuration_REQUIRED {
				state = HandshakeState_UPGRADE_SSL
				continue
			}

			state = HandshakeState_SEND_AUTHENTICATION
			break
		case HandshakeState_UPGRADE_SSL:
			req := &protocol.Greeting{
				Type: &Greeting_SSL_UPGRADE,
			}
			fmt.Printf("[Send Upgrade Request]: %+v\n", req)
			if e := self.WriteRequest(req); e != nil {
				state = HandshakeState_ERROR
				continue
			}

			//reset Buffer
			self.Buffer.Reset()
			cert, err := tls.LoadX509KeyPair("certs/client.pem", "certs/client.key")
			if err != nil {
				fmt.Printf("server: loadkeys: %s", err)
			}
			config := tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: true}
			conn := tls.Client(self.Conn, &config)
			if err := conn.Handshake(); err != nil {
				fmt.Printf("DAMEPO! %s\n", err)
				return err
			}
			fmt.Printf("SSL Handhsake OK!\n")

			self.Conn = conn
			state = HandshakeState_SEND_AUTHENTICATION
			break
		case HandshakeState_SEND_AUTHENTICATION:
			// パスワード送る
			req := &protocol.Greeting{
				Type: &Greeting_AUTHENTICATION,
				Authentication: &protocol.Greeting_Authentication{
					Password: []byte(self.Password),
				},
			}
			fmt.Printf("[Send Authentication]: %+v\n", req)
			if e := self.WriteRequest(req); e != nil {
				state = HandshakeState_ERROR
				continue
			}
			state = HandshakeState_WAIT_AUTHENTICATION_RESPONSE
			break
		case HandshakeState_WAIT_AUTHENTICATION_RESPONSE:
			// Authetication_OKをまつ
			ack := &protocol.Greeting{}
			self.ReadGreeting(ack)
			fmt.Printf("[Authentication Response]: %+v\n", ack)
			if ack.GetType() != Greeting_AUTHENTICATION_OK {
				state = HandshakeState_ERROR
				continue
			}
			state = HandshakeState_PROCESS_READY
			break
		case HandshakeState_PROCESS_READY:
			// Optionコマンドは受け取れる
			ack := &protocol.Greeting{}
			self.ReadGreeting(ack)
			fmt.Printf("[Received Ready]: %+v\n", ack)

			if ack.GetType() == Greeting_MESSAGE_OPTION {
				// もういっかい！
				continue
			} else if ack.GetType() == Greeting_COMMAND_READY {
				state = HandshakeState_FINISHED
				continue
			} else {
				state = HandshakeState_ERROR
				continue
			}
			break
		case HandshakeState_ERROR:
			return errors.New("handshake failed")
			break
		case HandshakeState_FINISHED:
			break
		default:
			return errors.New(fmt.Sprintf("Unsupported state: %d", state))
		}

		if state == HandshakeState_FINISHED{
			fmt.Printf("==== Ready ====\n")
			break
		}
	}

	return nil
}
