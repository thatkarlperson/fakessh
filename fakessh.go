package main

import (
  "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
  "io/ioutil"
  "fmt"
  "net"
  "log"
)

var (
  accounts = map[string]string {
    "testuser": "tiger",
    "john": "mary",
    "mary": "john",
    "dave": "joe",
    "joe": "dave",
    "root": "qm22",
  }
)


func main() {
  // Configure server.
  config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
      addr := c.RemoteAddr().String()
      user := c.User()
      goodpass, ok := accounts[user]
      if ok && goodpass == string(pass) {
        log.Printf("Successful login for %s @ %s", user, addr)
        return nil, nil
      }
      log.Printf("Failed login: %s : %s @ %s\n", user, pass, addr)
      return nil, fmt.Errorf("password rejected for %q", user)
		},
	}

  // Load keys.
  keyed := false
  for _, keyfile := range([]string{"id_rsa", "id_dsa", "id_ecdsa"}) {
    privateBytes, err := ioutil.ReadFile(keyfile)
    if err != nil {
      log.Println("Failed to load private key:", keyfile)
      break
    }
    private, err := ssh.ParsePrivateKey(privateBytes)
    if err != nil {
      log.Println("Failed to parse private key:", keyfile)
      break
    }
    config.AddHostKey(private)
    log.Println("Added key:", keyfile)
    keyed = true
  }
  if !keyed {
    panic("No key, can't start.")
  }

  // Ready to listen.
  log.Print("Listening!")
	listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		panic("failed to listen for connection")
	}

  // Loop forever waiting for connections.
  for {
    nConn, err := listener.Accept()
    if err != nil {
      log.Print("Couldn't accept incoming connection: ", err)
    }
    go handleConn(nConn, config)
  }
}

func handleConn(nConn net.Conn, config *ssh.ServerConfig) {
  addr := nConn.RemoteAddr().String()
  log.Printf("Connection opened from %s", addr)
  defer log.Printf("Connection closed from %s", addr)

  // Negotiate SSH.  This is where passwords are checked.
  _, chans, _, err := ssh.NewServerConn(nConn, config)
  if err != nil {
    log.Printf("Couldn't handshake %s: %s", addr, err)
  }

	// A ServerConn multiplexes several channels, which must
	// themselves be Accepted.
	for newChannel := range chans {
		// Accept reads from the connection, demultiplexes packets
		// to their corresponding channels and returns when a new
		// channel request is seen. Some goroutine must always be
		// calling Accept; otherwise no messages will be forwarded
		// to the channels.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, _, err := newChannel.Accept()
		if err != nil {
      break
		}

		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
    log.Printf("Channel from %s : %s", addr, newChannel.ChannelType())

    prompt := "$ "
		term := terminal.NewTerminal(channel, prompt)
		// serverTerm := &ssh.ServerTerminal{
		// 	Term:    term,
		// 	Channel: channel,
		// }
		go func() {
      defer log.Printf("Channel closed from %s", addr)
			defer channel.Close()
			for {
				line, err := term.ReadLine()
				if err != nil {
					break
				}
				log.Printf("%s > %s", addr, line)
			}
		}()
	}
}
