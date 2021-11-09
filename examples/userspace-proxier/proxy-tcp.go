package main

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type tcpProxy struct {
	svc           *service
	localAddrPort string
	targetPort    string
}

// Start proxying and return the lsnr to be closed when the proxy is removed
func (proxy tcpProxy) Start() io.Closer {
	lsnr, err := net.Listen("tcp", proxy.localAddrPort)
	if err != nil {
		log.Print("warning: failed to listen on ", proxy.localAddrPort, ": ", err)
		return nil
	}

	logPrefix := "tcp://" + proxy.localAddrPort

	log.Print("listening on ", logPrefix)

	go func() {
		for {
			conn, err := lsnr.Accept()
			if err != nil {
				log.Print(logPrefix, ": listener terminated: ", err)
				return
			}

			go proxy.handleConn(logPrefix, conn)
		}
	}()

	return lsnr
}

// handleConn handles an incoming client connection
func (proxy tcpProxy) handleConn(logPrefix string, conn net.Conn) {
	defer conn.Close()

	targetIP := proxy.svc.RandomEndpoint()
	if targetIP == "" {
		log.Print(logPrefix, ": service ", proxy.svc.Name, " has no endpoints")
		return
	}

	ipPort := net.JoinHostPort(targetIP, proxy.targetPort)

	log.Print(logPrefix, ": connecting client ", conn.RemoteAddr(), " to ", ipPort)

	tgt, err := net.DialTimeout("tcp", ipPort, 30*time.Second)
	if err != nil {
		log.Print(logPrefix, ": failed to connect to ", ipPort, ": ", err)
		return
	}

	defer tgt.Close()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(tgt, conn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(conn, tgt)
	}()

	wg.Wait()
}
