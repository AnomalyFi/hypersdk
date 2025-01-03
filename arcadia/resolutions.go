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
			if err := cli.Subscribe(); err != nil {
				time.Sleep(2 * time.Second)
				cli.vm.Logger().Error("failed to resubscribe to arcadia, waiting 2s to retry", zap.Error(err))
			} else {
				cli.isConnected = true
				cli.vm.Logger().Info("reconnected to arcadia")
				return
			}
		}
	}
}

func (cli *Arcadia) ShutDown() {
	cli.stopCalled = true
	if cli.isConnected {
		cli.conn.Close()
	}
	close(cli.stop)
}
