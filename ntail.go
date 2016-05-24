package main

import (
	"fmt"
	"hash/adler32"
	"io"
	"os"
	"path/filepath"

	"github.com/hpcloud/tail"
)

var DefaultLines uint = 10

type AnsiColor int

const (
	ColorBlack AnsiColor = 30 + iota
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite

	AutoColor AnsiColor = -1
	NoColor   AnsiColor = 0
)

var Colors = []AnsiColor{
	ColorRed,
	ColorGreen,
	ColorYellow,
	ColorBlue,
	ColorMagenta,
	ColorCyan,
}

type NTailConfig struct {
	tail.Config
	Lines         uint
	PrintFilename bool
	FilenameWidth int
	Color         AnsiColor
}

type NTail struct {
	*tail.Tail
	Config NTailConfig
}

func NewNTail(filename string, config NTailConfig) (*NTail, error) {
	f, err := tail.OpenFile(filename)
	if err != nil && !os.IsNotExist(err) {
		if !config.ReOpen {
			return nil, err
		}
	}
	f.Close()

	if config.Lines == 0 {
		config.Lines = DefaultLines
	}

	pos, err := lastLinesPos(filename, config.Lines)
	if err != nil {
		if config.ReOpen {
			pos = 0
		} else {
			return nil, err
		}
	}
	config.Location = &tail.SeekInfo{
		Offset: pos,
		Whence: os.SEEK_SET,
	}

	t, err := tail.TailFile(filename, config.Config)
	if err != nil {
		return nil, err
	}

	if config.FilenameWidth <= 0 {
		config.FilenameWidth = len(filename)
	}

	if config.Color == AutoColor {
		config.Color = color(filepath.Base(filename))
	}

	return &NTail{
		Tail:   t,
		Config: config,
	}, nil
}

func NewNTails(filenames []string, config NTailConfig, consistentColor bool) ([]*NTail, error) {
	if config.FilenameWidth <= 0 {
		config.FilenameWidth = maxBaseLength(filenames)
	}

	var ntails []*NTail
	for i, filename := range filenames {
		c := config
		if c.Color == AutoColor && !consistentColor {
			c.Color = Colors[i%len(Colors)]
		}

		nt, err := NewNTail(filename, c)
		if err != nil {
			return nil, err
		}
		ntails = append(ntails, nt)
	}
	return ntails, nil
}

func (nt *NTail) Print(w io.Writer) error {
	basename := filepath.Base(nt.Tail.Filename)
	header := nt.header(basename)

	for line := range nt.Lines {
		if line.Err != nil {
			return line.Err
		}
		_, err := fmt.Fprintf(w, "%s%s\n", header, line.Text)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nt *NTail) header(basename string) string {
	if !nt.Config.PrintFilename {
		return ""
	}

	if nt.Config.Color != NoColor {
		return fmt.Sprintf("\x1b[%dm%*s\x1b[0m: ", nt.Config.Color, nt.Config.FilenameWidth, basename)
	}
	return fmt.Sprintf("%*s: ", nt.Config.FilenameWidth, basename)
}

func color(basename string) AnsiColor {
	s := adler32.Checksum([]byte(basename))
	return Colors[s%uint32(len(Colors))]
}

func maxBaseLength(filenames []string) int {
	max := 0
	for _, filename := range filenames {
		len := len(filepath.Base(filename))
		if len > max {
			max = len
		}
	}
	return max
}

func lastLinesPos(filename string, n uint) (int64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	s, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if s.Size() == 0 {
		return 0, nil
	}

	pos, err := f.Seek(-1, os.SEEK_END)
	if err != nil {
		return 0, err
	}
	pos = s.Size()

	c := int(n)
	buf := make([]byte, 1)
	for pos > 0 && c >= 0 {
		pos, err = f.Seek(pos-1, os.SEEK_SET)
		if err != nil {
			return 0, err
		}

		_, err := f.Read(buf)
		if err != nil {
			return 0, err
		}
		if buf[0] == '\n' {
			c--
		}
	}

	return pos + 1, nil
}
