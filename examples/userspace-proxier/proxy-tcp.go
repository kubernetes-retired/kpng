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

func (proxy tcpProxy) handleConn(logPrefix string, conn net.Conn) {
	defer conn.Close()

	ep := proxy.svc.RandomEndpoint()
	if ep == nil {
		log.Print(logPrefix, ": service ", proxy.svc.Name, " has no endpoints")
		return
	}

	ipPort := net.JoinHostPort(ep.IPs.First(), proxy.targetPort)

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
