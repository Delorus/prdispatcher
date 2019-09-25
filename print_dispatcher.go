package printDispatcher_go

import (
	"log"
	"runtime"
	"sync/atomic"
	"time"
)

type PrinterFile interface {
	PaperSize() int
	TypeName() string
	PrintDuration() time.Duration
}

type PrintDispatcher interface {
	Stop() []PrinterFile
	Print(file PrinterFile)
	Cancel() PrinterFile
	PrintedFile() []PrinterFile
	CalcAvgPrintDuration() time.Duration
}

type Printer interface {
	Print(file PrinterFile) error
	Cancel() PrinterFile
}

type defaultPrintDispatcher struct {
	fileQueue    chan PrinterFile
	controlQueue chan controlAction

	stopped         int32
	notPrintedFiles []PrinterFile
}

type controlAction func(*context)

type context struct {
	printer Printer

	printedCh chan PrinterFile
	printed   []PrinterFile

	printQuota chan struct{}
}

func NewPrintDispatcher(printer Printer) PrintDispatcher {
	disp := &defaultPrintDispatcher{
		fileQueue:    make(chan PrinterFile, 10),
		controlQueue: make(chan controlAction),
	}

	go func() {
		ctx := &context{
			printer:    printer,
			printedCh:  make(chan PrinterFile, 1),
			printQuota: make(chan struct{}, 1),
		}

		ctx.printQuota <- struct{}{}
		for {
			select {
			case file := <-disp.fileQueue:
				log.Printf("get new file: %s", file)
				go func() {
					log.Printf("wait for printing: %s", file)
					<-ctx.printQuota
					defer func() { ctx.printQuota <- struct{}{} }()

					log.Printf("printing: %s", file)
					if err := ctx.printer.Print(file); err == nil {
						ctx.printedCh <- file
						log.Printf("successful printed: %s", file)
					}
				}()
				runtime.Gosched()

			case printedFile := <-ctx.printedCh:
				log.Printf("add file to printed array: %s", printedFile)
				ctx.printed = append(ctx.printed, printedFile)

			case action, ok := <-disp.controlQueue:
				if !ok {
					log.Print("stopped printer")
					return
				}

				log.Println("perform control action...")
				action(ctx)
				log.Println("control action performed")
			}
		}
	}()

	return disp
}

func (d *defaultPrintDispatcher) Stop() []PrinterFile {
	active := atomic.CompareAndSwapInt32(&d.stopped, 0, 1)
	if !active {
		log.Print("printer already stopped")
		return d.notPrintedFiles
	}
	d.Cancel()
	close(d.controlQueue)

	d.notPrintedFiles = make([]PrinterFile, 0, len(d.fileQueue))

FetchNotPrintedFiles:
	for {
		select {
		case file := <-d.fileQueue:
			d.notPrintedFiles = append(d.notPrintedFiles, file)
		default:
			break FetchNotPrintedFiles
		}
	}

	return d.notPrintedFiles
}

func (d *defaultPrintDispatcher) Print(file PrinterFile) {
	stopped := atomic.LoadInt32(&d.stopped)
	if stopped == 0 {
		log.Printf("put file in queue: %s", file)
		d.fileQueue <- file
	}
}

func (d *defaultPrintDispatcher) Cancel() PrinterFile {
	stopped := atomic.LoadInt32(&d.stopped)
	if stopped == 1 {
		log.Println("cannot cancel, printer already stopped")
		return nil
	}

	log.Println("cancel active printing")
	currentFile := make(chan PrinterFile)
	d.controlQueue <- func(ctx *context) {
		currentFile <- ctx.printer.Cancel()
	}

	return <-currentFile
}

func (d *defaultPrintDispatcher) PrintedFile() []PrinterFile {
	stopped := atomic.LoadInt32(&d.stopped)
	if stopped == 1 {
		return nil
	}

	log.Printf("get printed files...")
	printedFiles := make(chan []PrinterFile, 1)
	d.controlQueue <- func(ctx *context) {
		printedFiles <- ctx.printed
	}

	return <-printedFiles
}

func (d *defaultPrintDispatcher) CalcAvgPrintDuration() time.Duration {
	files := d.PrintedFile()
	log.Printf("calc avg duration of files: %v", files)

	count := len(files)
	var sum time.Duration
	for _, file := range files {
		sum += file.PrintDuration()
	}

	return time.Duration(int64(sum) / int64(count))
}
