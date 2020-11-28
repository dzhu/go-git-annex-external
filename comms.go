package helper

import (
	"fmt"
	"strconv"
	"strings"
)

func (r *remoteRunner) getLine() string {
	switch {
	case !r.scanner.Scan():
		return ""
	case r.scanner.Err() != nil:
		panic(r.scanner.Err())
	default:
		Log("\x1b[34m<- %s\x1b[m", r.scanner.Text())
		return r.scanner.Text()
	}
}

func (r *remoteRunner) sendLine(cmd string, args ...interface{}) {
	line := strings.TrimRight(fmt.Sprintln(append([]interface{}{cmd}, args...)...), "\n")
	line = strings.ReplaceAll(line, "\n", "\\n")
	Log("\x1b[32m-> %s\x1b[m", line)
	if _, err := fmt.Fprintln(r.output, line); err != nil {
		panic(err)
	}
}

func (r *remoteRunner) sendSuccess(cmd string, args ...interface{}) {
	r.sendLine(cmd+"-SUCCESS", args...)
}

func (r *remoteRunner) sendFailure(cmd string, args ...interface{}) {
	r.sendLine(cmd+"-FAILURE", args...)
}

func (r *remoteRunner) sendUnknown(cmd string, args ...interface{}) {
	r.sendLine(cmd+"-UNKNOWN", args...)
}

func (r *remoteRunner) ask(cmd string, args ...interface{}) string {
	r.sendLine(cmd, args...)
	resp := r.getLine()
	sp := strings.SplitN(resp, " ", 2)
	if sp[0] != "VALUE" {
		panic(fmt.Sprintf("got %s rather than VALUE in response", sp[0]))
	}
	return sp[1]
}

func (r *remoteRunner) unsupported() {
	r.sendLine("UNSUPPORTED-REQUEST")
}

func (r *remoteRunner) Progress(bytes int) {
	r.sendLine("PROGRESS", strconv.Itoa(bytes))
}

func (r *remoteRunner) DirHash(key string) string {
	return r.ask("DIRHASH", key)
}

func (r *remoteRunner) DirHashLower(key string) string {
	return r.ask("DIRHASH-LOWER", key)
}

func (r *remoteRunner) SetConfig(setting, value string) {
	r.sendLine("SETCONFIG", setting, value)
}

func (r *remoteRunner) GetConfig(setting string) string {
	return r.ask("GETCONFIG", setting)
}

func (r *remoteRunner) SetCreds(setting, user, password string) {
	r.sendLine("SETCREDS", setting, user, password)
}

func (r *remoteRunner) GetCreds(setting string) (string, string) {
	r.sendLine("GETCREDS", setting)
	resp := r.getLine()
	sp := strings.SplitN(resp, " ", 3)
	if sp[0] != "CREDS" {
		panic(fmt.Sprintf("got %s rather than CREDS in response", sp[0]))
	}
	return sp[1], sp[2]
}

func (r *remoteRunner) GetUUID() string {
	return r.ask("GETUUID")
}

func (r *remoteRunner) GetGitDir() string {
	return r.ask("GETGITDIR")
}

func (r *remoteRunner) SetWanted(expression string) {
	r.sendLine("SETWANTED", expression)
}

func (r *remoteRunner) GetWanted() string {
	return r.ask("GETWANTED")
}

func (r *remoteRunner) SetState(setting, value string) {
	r.sendLine("SETSTATE", setting, value)
}

func (r *remoteRunner) GetState(setting string) string {
	return r.ask("GETSTATE", setting)
}

func (r *remoteRunner) SetURLPresent(key, url string) {
	r.sendLine("SETURLPRESENT", key, url)
}

func (r *remoteRunner) SetURLMissing(key, url string) {
	r.sendLine("SETURLMISSING", key, url)
}

func (r *remoteRunner) SetURIPresent(key, uri string) {
	r.sendLine("SETURIPRESENT", key, uri)
}

func (r *remoteRunner) SetURIMissing(key, uri string) {
	r.sendLine("SETURIMISSING", key, uri)
}

func (r *remoteRunner) GetURLs(key, prefix string) []string {
	r.sendLine("GETURLS", key, prefix)
	var urls []string
	for line := r.getLine(); line != "VALUE "; line = r.getLine() {
		urls = append(urls, strings.SplitN(line, " ", 2)[1])
	}
	return urls
}

func (r *remoteRunner) Debug(message string) {
	r.sendLine("DEBUG", message)
}

func (r *remoteRunner) Debugf(format string, args ...interface{}) {
	r.Debug(fmt.Sprintf(format, args...))
}

func (r *remoteRunner) Info(message string) {
	r.sendLine("INFO", message)
}

func (r *remoteRunner) Infof(format string, args ...interface{}) {
	r.Info(fmt.Sprintf(format, args...))
}

func (r *remoteRunner) Error(message string) {
	r.sendLine("ERROR", message)
}

func (r *remoteRunner) Errorf(format string, args ...interface{}) {
	r.Error(fmt.Sprintf(format, args...))
}
