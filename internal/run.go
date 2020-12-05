package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	// StartupCmd is a command value that is always sent once as the first command.
	StartupCmd = "__internal_startup__"
	// UnsupportedCmd is a command value that is sent when an unsupported command is received.
	UnsupportedCmd = "__internal_unsupported__"
)

type LineIO interface {
	Send(cmd string, args ...interface{})
	Recv() string
}

func fmtLine(cmd string, args ...interface{}) string {
	line := strings.TrimRight(fmt.Sprintln(append([]interface{}{cmd}, args...)...), "\n")
	return strings.ReplaceAll(line, "\n", "\\n")
}

type rawLineIO struct {
	w io.Writer
	s *bufio.Scanner
}

func (r *rawLineIO) Send(cmd string, args ...interface{}) {
	line := fmtLine(cmd, args...)
	if _, err := fmt.Fprintln(r.w, line); err != nil {
		panic(err)
	}
}

func (r *rawLineIO) Recv() string {
	switch {
	case !r.s.Scan():
		return ""
	case r.s.Err() != nil:
		panic(r.s.Err())
	default:
		return r.s.Text()
	}
}

type CommandSpec struct {
	ArgCount int
	Response func(args []string)
}

func procLine(cmds map[string]CommandSpec, line string) {
	cmdAndArgs := strings.SplitN(line, " ", 2)
	cmd := cmdAndArgs[0]

	spec, ok := cmds[cmd]
	if !ok {
		if cmd != StartupCmd && cmd != UnsupportedCmd {
			procLine(cmds, UnsupportedCmd)
		}
		return
	}
	argsStr := ""
	if len(cmdAndArgs) > 1 {
		argsStr = cmdAndArgs[1]
	}
	args := strings.SplitN(argsStr, " ", spec.ArgCount)
	spec.Response(args)
}

func runJob(lines LineIO, cmds map[string]CommandSpec) {
	for line := lines.Recv(); line != ""; line = lines.Recv() {
		procLine(cmds, line)
	}
}

func getJobNum(line string) int {
	split := strings.SplitN(line, " ", 3)
	if split[0] != "J" {
		return 0
	}
	jobNum, _ := strconv.Atoi(split[1])
	return jobNum
}

type jobLineIO struct {
	input  <-chan string
	num    int
	output chan<- string
}

func (j *jobLineIO) needsJobPrefix() bool {
	return j.num != 0
}

func (j *jobLineIO) Send(cmd string, args ...interface{}) {
	if cmd != "ERROR" && j.needsJobPrefix() {
		args = append([]interface{}{j.num, cmd}, args...)
		cmd = "J"
	}
	j.output <- fmtLine(cmd, args...)
}

func (j *jobLineIO) Recv() string {
	line := <-j.input
	if line == "" || !j.needsJobPrefix() {
		return line
	}
	prefix := fmt.Sprintf("J %d ", j.num)
	rest := strings.TrimPrefix(line, prefix)
	if line == rest {
		panic(fmt.Sprintf("received line %q without correct prefix %q", line, prefix))
	}
	return rest
}

// RunWithStreams executes an external special remote with the provided input and output streams.
func RunWithStreams(in io.Reader, out io.Writer, makeCmds func(LineIO) map[string]CommandSpec) {
	lines := &rawLineIO{
		w: out,
		s: bufio.NewScanner(in),
	}
	defer func() {
		if p := recover(); p != nil {
			lines.Send("ERROR", "failed:", p)
		}
	}()

	outLines := make(chan string)

	go func() {
		for l := range outLines {
			lines.Send(l)
		}
	}()

	inChans := make(map[int]chan string)

	for line := StartupCmd; line != ""; line = lines.Recv() {
		jobNum := getJobNum(line)
		ch, ok := inChans[jobNum]
		if !ok {
			ch = make(chan string)
			inChans[jobNum] = ch
			j := &jobLineIO{ch, jobNum, outLines}
			go runJob(j, makeCmds(j))
		}
		ch <- line
	}
}

func Run(makeCmds func(LineIO) map[string]CommandSpec) {
	RunWithStreams(os.Stdin, os.Stdout, makeCmds)
}
