package printDispatcher_go_test

import (
	"errors"
	"fmt"
	"log"
	disp "printDispatcher-go"
	"strconv"
	"testing"
	"time"
)

func TestDefaultPrintDispatcher_Print(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	expectedDoc := MockDocument{
		paperSize:     42,
		typeName:      "test document",
		printDuration: 0,
	}

	dispatcher.Print(expectedDoc)

	assertEqualsDoc(t, expectedDoc, printer.CurrentFile())
}

func TestDefaultPrintDispatcher_PrintMultipleFiles(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	docs := make([]disp.PrinterFile, 10)
	expectedDocs := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		docs[i] = MockDocument{
			paperSize:     42,
			typeName:      strconv.Itoa(i),
			printDuration: 0,
		}
		expectedDocs[docs[i].TypeName()] = struct{}{}
	}

	for _, doc := range docs {
		dispatcher.Print(doc)
	}

	for i := 0; i < 10; i++ {
		actual := printer.CurrentFile()
		if _, exist := expectedDocs[actual.TypeName()]; exist {
			delete(expectedDocs, actual.TypeName())
		} else {
			t.Fatalf("not expected, but printed file: %s", actual)
		}
	}
	if len(expectedDocs) != 0 {
		for typeName := range expectedDocs {
			t.Errorf("not printed, but expected file: %s", typeName)
		}
	}
}

func TestDefaultPrintDispatcher_StopInFilledQueue(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	expectedDoc := MockDocument{
		paperSize:     42,
		typeName:      "test document",
		printDuration: 0,
	}
	dispatcher.Print(expectedDoc)

	notPrintedDocs := dispatcher.Stop()

	if len(notPrintedDocs) != 1 {
		t.Fatalf("incorrect count of not printed files: %d", len(notPrintedDocs))
	}
	assertEqualsDoc(t, expectedDoc, notPrintedDocs[0])
}

func TestDefaultPrintDispatcher_StopEmptyQueue(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)

	notPrintedDocs := dispatcher.Stop()

	if len(notPrintedDocs) != 0 {
		t.Fatalf("incorrect count of not printed files: %d", len(notPrintedDocs))
	}
}

func TestDefaultPrintDispatcher_Cancel(t *testing.T) {
	log.Printf("start: %s", t.Name())
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	expectedDoc := MockDocument{
		paperSize:     42,
		typeName:      "test document",
		printDuration: 0,
	}
	dispatcher.Print(expectedDoc)
	if err := printer.WaitForPrinting(); err != nil {
		t.Fail()
	}

	actual := dispatcher.Cancel()

	assertEqualsDoc(t, expectedDoc, actual)
	log.Printf("end: %s", t.Name())
}

func TestDefaultPrintDispatcher_PrintedFile(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	printed := MockDocument{
		paperSize:     42,
		typeName:      "printed document",
		printDuration: 0,
	}
	notPrinted := MockDocument{
		paperSize:     42,
		typeName:      "not printed document",
		printDuration: 0,
	}
	dispatcher.Print(printed)
	dispatcher.Print(notPrinted)
	printer.PrintCurrentFile()
	if err := printer.WaitForPrinting(); err != nil {
		t.Fail()
	}

	printedDocs := dispatcher.PrintedFile()

	if len(printedDocs) != 1 {
		t.Fatalf("incorrect count of printed files: %d", len(printedDocs))
	}
	assertEqualsDoc(t, printed, printedDocs[0])
}

func TestDefaultPrintDispatcher_PrintedFileEmpty(t *testing.T) {
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)

	printedDocs := dispatcher.PrintedFile()

	if len(printedDocs) != 0 {
		t.Fatalf("incorrect count of printed files: %d", len(printedDocs))
	}
}

func TestDefaultPrintDispatcher_CalcAvgPrintDuration(t *testing.T) {
	log.Printf("start: %s", t.Name())
	printer := NewMockPrinter()
	dispatcher := disp.NewPrintDispatcher(printer)
	first := MockDocument{
		paperSize:     42,
		typeName:      "first",
		printDuration: 1 * time.Second,
	}
	second := MockDocument{
		paperSize:     42,
		typeName:      "second",
		printDuration: 2 * time.Second,
	}
	notPrinted := MockDocument{
		paperSize:     42,
		typeName:      "not printed",
		printDuration: 0,
	}
	dispatcher.Print(first)
	dispatcher.Print(second)
	dispatcher.Print(notPrinted)
	printer.PrintCurrentFile()
	printer.PrintCurrentFile()
	if err := printer.WaitForPrinting(); err != nil {
		t.Fail()
	}

	avg := dispatcher.CalcAvgPrintDuration()

	if avg != 1500*time.Millisecond {
		t.Errorf("incorrect avg duration: %s", avg)
	}
	log.Printf("end: %s", t.Name())
}

func assertEqualsDoc(t *testing.T, expected, actual disp.PrinterFile) {
	if expected != actual {
		t.Errorf("\nexpected: %s\ngot: %s", expected, actual)
	}
}

type MockDocument struct {
	paperSize     int
	typeName      string
	printDuration time.Duration
}

func (m MockDocument) String() string {
	return fmt.Sprintf("[type: %s, size: %d, duration: %s]", m.typeName, m.paperSize, m.printDuration)
}

func (m MockDocument) PaperSize() int {
	return m.paperSize
}

func (m MockDocument) TypeName() string {
	return m.typeName
}

func (m MockDocument) PrintDuration() time.Duration {
	return m.printDuration
}

func NewMockPrinter() *MockPrinter {
	return &MockPrinter{
		ch:   make(chan disp.PrinterFile),
		wait: make(chan struct{}, 1),
	}
}

type MockPrinter struct {
	ch   chan disp.PrinterFile
	wait chan struct{}
}

func (m *MockPrinter) WaitForPrinting() error {
	select {
	case <-m.wait:
		return nil
	case <-time.After(100 * time.Millisecond):
		return errors.New("time out")
	}
}

func (m *MockPrinter) Print(file disp.PrinterFile) error {
	select {
	case m.wait <- struct{}{}:
	default:
	}
	m.ch <- file
	return nil
}

func (m *MockPrinter) Cancel() disp.PrinterFile {
	select {
	case file := <-m.ch:
		return file
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (m *MockPrinter) PrintCurrentFile() {
	m.CurrentFile()
}

func (m *MockPrinter) CurrentFile() disp.PrinterFile {
	select {
	case current := <-m.ch:
		return current
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}
