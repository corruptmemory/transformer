package commands

import (
	"context"
	"encoding/gob"
	"log"
	"net"
	"sync"
)

func (c *Commands) StartRemoteControlSession(socketPath string) (err error) {
	return c.WithGroupInScopeErr("StartRemoteControlSession", func(c *Commands) (err error) {
		wg := &sync.WaitGroup{}
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		listener, err := net.Listen("unix", socketPath)
		defer cancel()
		if err != nil {
			c.Errorf("Error setting up listener:", err)
			return
		}
		defer func() {
			listener.Close()
		}()
	stopAcceptingConnections:
		for {
			select {
			case <-ctx.Done():
				break stopAcceptingConnections
			default:
				conn, err := listener.Accept()
				if err != nil {
					c.Errorf("Error accepting connection:", err)
					continue
				}
				wg.Add(1)
				go c.handleConnection(conn, cancel, wg)
			}
		}

		wg.Wait()
		return nil
	})

}

func (c *Commands) handleConnection(conn net.Conn, cancelFunc context.CancelFunc, wg *sync.WaitGroup) {
	defer func() {
		conn.Close()
		wg.Done()
	}()

	enc := gob.NewEncoder(conn) // Will write to network.
	dec := gob.NewDecoder(conn)
	if err != nil {
		log.Fatal("Error reading message:", err)
	}

	log.Println("Received message:", string(msg))
}
