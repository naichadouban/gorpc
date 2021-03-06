package rotator

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// nl is a byte slice containing a newline byte.  It is used to avoid creating
// additional allocations when writing newlines to the log file.
var nl = []byte{'\n'}
// A Rotator writes input to a file, splitting it up into gzipped chunks once
// the filesize reaches a certain threshold.
type Rotator struct {
	size      int64
	threshold int64
	maxRolls  int
	filename  string
	out       *os.File
	tee       bool
	wg        sync.WaitGroup
}

// New returns a new Rotator.  The rotator can be used either by reading input
// from an io.Reader by calling Run, or writing directly to the Rotator with
// Write.
// 从一个reader中读调用Run,直接写调用Writer
// New returns a new Rotator.  The rotator can be used either by reading input
// from an io.Reader by calling Run, or writing directly to the Rotator with
// Write.
func New(filename string, thresholdKB int64, tee bool, maxRolls int) (*Rotator, error) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return &Rotator{
		size:      stat.Size(),
		threshold: 1000 * thresholdKB,
		maxRolls:  maxRolls,
		filename:  filename,
		out:       f,
		tee:       tee,
	}, nil
}

// Run begins reading lines from the reader and rotating logs as necessary.  Run
// should not be called concurrently with Write.
//
// Prefer to use Rotator as a writer instead to avoid unnecessary scanning of
// input, as this job is better handled using io.Pipe.
func (r *Rotator) Run(reader io.Reader) error {
	in := bufio.NewReader(reader)

	// Rotate file immediately if it is already over the size limit.
	if r.size >= r.threshold {
		if err := r.rotate(); err != nil {
			return err
		}
		r.size = 0
	}

	for {
		line, isPrefix, err := in.ReadLine()
		if err != nil {
			return err
		}

		n, _ := r.out.Write(line)
		r.size += int64(n)
		if r.tee {
			os.Stdout.Write(line)
		}
		if isPrefix {
			continue
		}

		m, _ := r.out.Write(nl)
		if r.tee {
			os.Stdout.Write(nl)
		}
		r.size += int64(m)

		if r.size >= r.threshold {
			err := r.rotate()
			if err != nil {
				return err
			}
			r.size = 0
		}
	}
}

// Write implements the io.Writer interface for Rotator.  If p ends in a newline
// and the file has exceeded the threshold size, the file is rotated.
func (r *Rotator) Write(p []byte) (n int, err error) {
	n, _ = r.out.Write(p)
	r.size += int64(n)

	if r.size >= r.threshold && len(p) > 0 && p[len(p)-1] == '\n' {
		err := r.rotate()
		if err != nil {
			return 0, err
		}
		r.size = 0
	}

	return n, nil
}

// 无论是Run还是Write都需要调用rotate
func (r *Rotator) rotate() error {
	dir := filepath.Dir(r.filename)
	glob := filepath.Join(dir, filepath.Base(r.filename)+".*")
	existing, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	maxNum := 0
	for _, name := range existing {
		parts := strings.Split(name, ".")
		if len(parts) < 2 {
			continue
		}
		numIdx := len(parts) - 1
		if parts[numIdx] == "gz" {
			numIdx--
		}
		num, err := strconv.Atoi(parts[numIdx])
		if err != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}

	err = r.out.Close()
	if err != nil {
		return err
	}
	rotname := fmt.Sprintf("%s.%d", r.filename, maxNum+1)
	err = os.Rename(r.filename, rotname)
	if err != nil {
		return err
	}
	if r.maxRolls > 0 {
		for n := maxNum + 1 - r.maxRolls; ; n-- {
			err := os.Remove(fmt.Sprintf("%s.%d.gz", r.filename, n))
			if err != nil {
				break
			}
		}
	}
	r.out, err = os.OpenFile(r.filename, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	r.wg.Add(1)
	go func() {
		err := compress(rotname)
		if err == nil {
			os.Remove(rotname)
		}
		r.wg.Done()
	}()

	return nil
}

// 压缩，rotator的时候调用
func compress(name string) (err error) {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	arc, err := os.OpenFile(name+".gz", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	z := gzip.NewWriter(arc)
	if _, err = io.Copy(z, f); err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	return arc.Close()
}
