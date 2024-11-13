package arcadia

func (cli *Arcadia) EpochUpdateChan() chan *EpochUpdateInfo {
	return cli.epochUpdatechan
}

func (cli *Arcadia) CurrEpochNameSpaces() *[][]byte {
	return cli.AvailableNamespaces
}
