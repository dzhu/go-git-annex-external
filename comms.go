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
	return strings.SplitN(resp, " ", 2)[1]
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

func (r *remoteRunner) GetUUID() string {
	return r.ask("GETUUID")
}

func (r *remoteRunner) Debug(message string) {
	r.sendLine("DEBUG", message)
}

func (r *remoteRunner) Info(message string) {
	r.sendLine("INFO", message)
}
