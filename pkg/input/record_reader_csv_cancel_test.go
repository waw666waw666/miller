package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johnkerl/miller/v6/pkg/cli"
	"github.com/johnkerl/miller/v6/pkg/types"
)

func TestCSVReadersStopBeforeLaterFiles(t *testing.T) {
	for _, format := range []string{"csv", "csvlite", "tsv"} {
		for _, cancel := range []bool{false, true} {
			name := format + "/complete"
			if cancel {
				name = format + "/cancelled"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				extension := ".csv"
				separator := ","
				if format == "tsv" {
					extension = ".tsv"
					separator = "\t"
				}
				first := filepath.Join(dir, "first"+extension)
				second := filepath.Join(dir, "second"+extension)
				data := "a" + separator + "b\n" + strings.Repeat("1"+separator+"2\n", 100)
				if err := os.WriteFile(first, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				if !cancel {
					if err := os.WriteFile(second, []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				options := cli.DefaultReaderOptions()
				options.InputFileFormat = format
				if err := cli.FinalizeReaderOptions(&options); err != nil {
					t.Fatal(err)
				}
				// Batch size 1 checks cancellation after every record/line and bounds
				// scanner read-ahead well below the 100-record first file. The
				// cancelled-case count bound below relies on this small batch size.
				reader, err := Create(&options, 1)
				if err != nil {
					t.Fatal(err)
				}
				records := make(chan []*types.RecordAndContext)
				errorChannel := make(chan error, 10)
				done := make(chan bool, 1)
				context := types.NewContext()
				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()
				go reader.Read([]string{first, second}, *context, records, errorChannel, done)
				count := 0
				sentDone := false
				for {
					var batch []*types.RecordAndContext
					select {
					case batch = <-records:
					case <-timer.C:
						t.Fatal("reader did not send an end-of-stream marker")
					}
					for _, record := range batch {
						if record.EndOfStream {
							if cancel && count >= 100 {
								t.Errorf("cancelled reader consumed all %d records", count)
							}
							if !cancel && count != 200 {
								t.Errorf("read %d records, want 200", count)
							}
							select {
							case err := <-errorChannel:
								t.Fatalf("unexpected later input error: %v", err)
							default:
							}
							return
						}
						count++
					}
					if cancel && !sentDone {
						done <- true
						sentDone = true
					}
				}
			})
		}
	}
}
