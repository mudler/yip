package plugins

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/mudler/yip/pkg/logger"
	"github.com/mudler/yip/pkg/schema"
	"github.com/twpayne/go-vfs"
)

func Commands(l logger.Interface, s schema.Stage, fs vfs.FS, console Console) error {
	var errs error
	var wg sync.WaitGroup
	wg.Add(len(s.Commands))
	resultCh := make(chan error)

	for _, cmd := range s.Commands {
		switch strings.ToLower(s.RunType) {
		case "ignore":
			_, _ = console.Run(templateSysData(l, cmd))
			continue
		case "default", "":
			out, err := console.Run(templateSysData(l, cmd))
			if err != nil {
				l.Error(out, ": ", err.Error())
				errs = multierror.Append(errs, err)
				continue
			}
			l.Info(fmt.Sprintf("Command output: %s", string(out)))
		case "background":
			go executeFunction(l, cmd, console, &wg, resultCh)
		}
	}
	if strings.ToLower(s.RunType) == "background" {
		// Wait for all goroutines to finish
		go func() {
			wg.Wait()
			close(resultCh)
		}()

		// Process the exit statuses of the functions
		for result := range resultCh {
			if result != nil {
				errs = multierror.Append(errs, result)
			}
		}
	}

	return errs
}

func executeFunction(l logger.Interface, cmd string, console Console, wg *sync.WaitGroup, resultCh chan<- error) {
	defer wg.Done()

	_, err := console.Run(templateSysData(l, cmd))
	if err != nil {
		resultCh <- err
	}
}
