package helper

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type lineIO interface {
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

type annexIO struct {
	io lineIO
}

func (a *annexIO) send(cmd string, args ...interface{}) {
	a.io.Send(cmd, args...)
}

func (a *annexIO) sendSuccess(cmd string, args ...interface{}) {
	a.send(cmd+"-SUCCESS", args...)
}

func (a *annexIO) sendFailure(cmd string, args ...interface{}) {
	a.send(cmd+"-FAILURE", args...)
}

func (a *annexIO) sendUnknown(cmd string, args ...interface{}) {
	a.send(cmd+"-UNKNOWN", args...)
}

func (a *annexIO) ask(cmd string, args ...interface{}) string {
	a.send(cmd, args...)
	resp := a.io.Recv()
	sp := strings.SplitN(resp, " ", 2)
	if sp[0] != "VALUE" {
		panic(fmt.Sprintf("got %s rather than VALUE in response", sp[0]))
	}
	return sp[1]
}

func (a *annexIO) unsupported() {
	a.send("UNSUPPORTED-REQUEST")
}

func (a *annexIO) Progress(bytes int) {
	a.send("PROGRESS", strconv.Itoa(bytes))
}

func (a *annexIO) DirHash(key string) string {
	return a.ask("DIRHASH", key)
}

func (a *annexIO) DirHashLower(key string) string {
	return a.ask("DIRHASH-LOWER", key)
}

func (a *annexIO) SetConfig(setting, value string) {
	a.send("SETCONFIG", setting, value)
}

func (a *annexIO) GetConfig(setting string) string {
	return a.ask("GETCONFIG", setting)
}

func (a *annexIO) SetCreds(setting, user, password string) {
	a.send("SETCREDS", setting, user, password)
}

func (a *annexIO) GetCreds(setting string) (string, string) {
	a.send("GETCREDS", setting)
	resp := a.io.Recv()
	sp := strings.SplitN(resp, " ", 3)
	if sp[0] != "CREDS" {
		panic(fmt.Sprintf("got %s rather than CREDS in response", sp[0]))
	}
	return sp[1], sp[2]
}

func (a *annexIO) GetUUID() string {
	return a.ask("GETUUID")
}

func (a *annexIO) GetGitDir() string {
	return a.ask("GETGITDIR")
}

func (a *annexIO) SetWanted(expression string) {
	a.send("SETWANTED", expression)
}

func (a *annexIO) GetWanted() string {
	return a.ask("GETWANTED")
}

func (a *annexIO) SetState(setting, value string) {
	a.send("SETSTATE", setting, value)
}

func (a *annexIO) GetState(setting string) string {
	return a.ask("GETSTATE", setting)
}

func (a *annexIO) SetURLPresent(key, url string) {
	a.send("SETURLPRESENT", key, url)
}

func (a *annexIO) SetURLMissing(key, url string) {
	a.send("SETURLMISSING", key, url)
}

func (a *annexIO) SetURIPresent(key, uri string) {
	a.send("SETURIPRESENT", key, uri)
}

func (a *annexIO) SetURIMissing(key, uri string) {
	a.send("SETURIMISSING", key, uri)
}

func (a *annexIO) GetURLs(key, prefix string) []string {
	a.send("GETURLS", key, prefix)
	var urls []string
	for line := a.io.Recv(); line != "VALUE "; line = a.io.Recv() {
		urls = append(urls, strings.SplitN(line, " ", 2)[1])
	}
	return urls
}

func (a *annexIO) Debug(message string) {
	a.send("DEBUG", message)
}

func (a *annexIO) Debugf(format string, args ...interface{}) {
	a.Debug(fmt.Sprintf(format, args...))
}

func (a *annexIO) Info(message string) {
	a.send("INFO", message)
}

func (a *annexIO) Infof(format string, args ...interface{}) {
	a.Info(fmt.Sprintf(format, args...))
}

func (a *annexIO) Error(message string) {
	a.send("ERROR", message)
}

func (a *annexIO) Errorf(format string, args ...interface{}) {
	a.Error(fmt.Sprintf(format, args...))
}
