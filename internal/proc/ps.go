package proc

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// parsePS parses `ps axo pid=,ppid=,rss=,comm=` output. comm comes last
// because executable paths may contain spaces; rss arrives in KiB.
func parsePS(out []byte) ([]Proc, error) {
	var procs []Proc
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		var nums [3]uint64
		ok := true
		for i := range nums {
			var field string
			field, line = nextField(line)
			n, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				ok = false
				break
			}
			nums[i] = n
		}
		comm := strings.TrimSpace(line)
		if !ok || comm == "" {
			continue
		}
		p := Proc{PID: int(nums[0]), PPID: int(nums[1]), RSS: nums[2] * 1024, Name: comm}
		if strings.HasPrefix(comm, "/") {
			p.Path = comm
			p.Name = filepath.Base(comm)
		}
		procs = append(procs, p)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parsing ps output: %w", err)
	}
	return procs, nil
}

// nextField returns the first whitespace-delimited token and the rest of the
// line. Splitting by position (not strings.Fields) keeps whitespace inside
// the trailing comm column intact.
func nextField(line string) (field, rest string) {
	line = strings.TrimLeft(line, " \t")
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, ""
	}
	return line[:i], line[i:]
}
