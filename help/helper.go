package help

import (
	"fmt"
	"os"
	"os/user"
)

func Dbg(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[DBG] "+format+"\n", a...)
}

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	// Windows fallback
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	return "." // last resort: current dir
}
