package arcadia

import (
	"time"

	"go.uber.org/zap"
)

func (cli *Arcadia) EpochUpdateChan() chan *EpochUpdateInfo {
	return cli.epochUpdatechan
}

func (cli *Arcadia) Reconnect() {
	for {
		select {
		case <-cli.stop:
			cli.vm.Logger().Info("receiving stop sig, stop subscribing")
			return
		default:
			if time.Since(cli.lastReconnect) < 2*time.Second {
				time.Sleep(2 * time.Second)
			}
			cli.lastReconnect = time.Now()
			if err := cli.Subscribe(); err != nil {
				time.Sleep(2 * time.Second)
				cli.vm.Logger().Error("failed to resubscribe to arcadia, waiting 2s to retry", zap.Error(err))
			} else {
				cli.isConnected.Store(true)
				cli.vm.Logger().Info("reconnected to arcadia")
				return
			}
		}
	}
}

func (cli *Arcadia) ShutDown() {
	if shutdowned := cli.stopCalled.Swap(true); shutdowned {
		return
	}

	if cli.isConnected.Load() {
		cli.conn.Close()
	}
	close(cli.stop)
}
