package arcadia

import (
	"time"

	"go.uber.org/zap"
)

func (cli *Arcadia) EpochUpdateChan() chan *EpochUpdateInfo {
	return cli.epochUpdatechan
}

func (cli *Arcadia) CurrEpochNameSpaces() *[][]byte {
	return cli.AvailableNamespaces
}

func (cli *Arcadia) Reconnect() {
	if cli.stopCalled {
		cli.vm.Logger().Info("not attempting reconnect as shutdown is called on arcadia")
		return
	}
	cli.vm.Logger().Info("reconnecting to arcadia")
	if err := cli.Subscribe(); err != nil {
		cli.vm.Logger().Error("failed to resubscribe to arcadia", zap.Error(err))
		// wait 2 minutes before retrying.
		cli.vm.Logger().Info("retrying to resubscribe to arcadia in 2 minutes")
		time.Sleep(120 * time.Second)
		if err := cli.Subscribe(); err != nil {
			cli.vm.Logger().Error("failed to resubscribe to arcadia", zap.Error(err))
			// wait 3 more minutes before retrying.
			cli.vm.Logger().Info("retrying to resubscribe to arcadia in 3 minutes x2")
			time.Sleep(180 * time.Second)
			if err := cli.Subscribe(); err != nil {
				cli.vm.Logger().Error("failed to resubscribe to arcadia", zap.Error(err))
				return
			}
		}
	}
	cli.vm.Logger().Info("resubscribed to arcadia")
}

func (cli *Arcadia) ShutDown() {
	cli.stopCalled = true
	cli.conn.Close()
	close(cli.stop)
}
