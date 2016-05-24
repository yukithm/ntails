package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/hpcloud/tail"
	"github.com/mattn/go-colorable"
)

var (
	logger          = log.New(os.Stderr, "", 0)
	follow          = flag.Bool("f", false, "tail to not stop when end of file is reached")
	reopen          = flag.Bool("F", false, "follow and keep trying to open a file")
	poll            = flag.Bool("poll", false, "polling instead of inotify")
	lines           = flag.Uint("n", 10, "output the last NUM lines")
	nofname         = flag.Bool("q", false, "suppresses printing of filenames")
	noColor         = flag.Bool("no-color", false, "disable color output")
	consistentColor = flag.Bool("consistent-color", false, "use consistent color by filename")
)

const helpText = `ntails v0.1.0

USAGE:
  ntails [OPTIONS] <FILE>...

OPTIONS:
`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, helpText)
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() == 0 {
		logger.Fatal("At least one file is required. (currently doesn't support STDIN)")
	}

	ntconf := NTailConfig{
		Config: tail.Config{
			Follow:    *follow || *reopen,
			ReOpen:    *reopen,
			Poll:      *poll,
			MustExist: !*reopen,
			Logger:    tail.DiscardingLogger,
		},
		Lines:         *lines,
		PrintFilename: flag.NArg() > 1 && !*nofname,
		Color:         AutoColor,
	}
	if *noColor {
		ntconf.Color = NoColor
	}

	var wg sync.WaitGroup
	ntails, err := NewNTails(flag.Args(), ntconf, *consistentColor)
	if err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		<-sig
		for _, nt := range ntails {
			nt.Stop()
		}
	}()

	var out io.Writer
	if *noColor {
		out = os.Stdout
	} else {
		out = colorable.NewColorableStdout()
	}

	for _, nt := range ntails {
		wg.Add(1)
		go func(nt *NTail) {
			err := nt.Print(out)
			if err != nil {
				log.Fatal(err)
			}
			wg.Done()
		}(nt)
	}
	wg.Wait()
}
