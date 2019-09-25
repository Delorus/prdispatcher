package printDispatcher_go

import (
	"errors"
	"time"
)

func NewDryPrinter() Printer {
	p := &DryPrinter{
		cancelCh: make(chan struct{}),
	}

	return p
}

type DryPrinter struct {
	cancelCh      chan struct{}
	cancelledFile chan PrinterFile
}

func (p *DryPrinter) Print(file PrinterFile) error {
	select {
	case <-time.After(file.PrintDuration()):
		return nil
	case <-p.cancelCh:
		p.cancelledFile <- file
		return errors.New("print has been canceled")
	}
}

func (p *DryPrinter) Cancel() PrinterFile {
	p.cancelCh <- struct{}{}
	return <-p.cancelledFile
}
