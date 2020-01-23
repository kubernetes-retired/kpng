package proxy

import (
	"os"
	"os/signal"
	"syscall"
)

func WaitForTermSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-ch
}
