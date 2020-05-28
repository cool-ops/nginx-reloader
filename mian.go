package main

import (
	"github.com/fsnotify/fsnotify"
	proc "github.com/shirou/gopsutil/process"
	"log"
	"os"
	"path/filepath"
	"syscall"
)

const (
	nginxProcessName     = "nginx"
	defaultNginxConfPath = "/etc/nginx"
	watchPathEnvVarName  = "WATCH_NGINX_CONF_PATH"
)

type nginxWatch struct {
	watcher *fsnotify.Watcher
}

var stderrLogger = log.New(os.Stderr, "error: ", log.Lshortfile)
var stdoutLogger = log.New(os.Stdout, "", log.Lshortfile)

// get nginx master pid
func (n nginxWatch) getMasterNginxPid() (int, error) {
	processes, processesErr := proc.Processes()
	if processesErr != nil {
		return 0, processesErr
	}
	nginxProcesses := map[int32]int32{}
	for _, process := range processes {
		processName, processNameErr := process.Name()
		if processNameErr != nil {
			return 0, processNameErr
		}
		if processName == nginxProcessName {
			ppid, ppidErr := process.Ppid()
			if ppidErr != nil {
				return 0, ppidErr
			}
			nginxProcesses[process.Pid] = ppid
		}
	}
	var masterNginxPid int32
	for pid, ppid := range nginxProcesses {
		if ppid == 0 {
			masterNginxPid = pid
			break
		}
	}
	stdoutLogger.Println("found master nginx pid:", masterNginxPid)
	return int(masterNginxPid), nil
}

// sign nginx reload
func (n nginxWatch) signalNginxReload(pid int) error {
	stdoutLogger.Printf("signaling master nginx process (pid: %d) -> SIGHUP\n", pid)
	nginxProcess, nginxProcessErr := os.FindProcess(pid)
	if nginxProcessErr != nil {
		return nginxProcessErr
	}
	return nginxProcess.Signal(syscall.SIGHUP)
}

// watch dir and sub dir
func (n nginxWatch) watchDir(dir string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			path, err := filepath.Abs(path)
			if err != nil {
				stdoutLogger.Println(err)
				return err
			}
			stdoutLogger.Printf("adding path: `%s` to watch\n", path)
			if err := n.watcher.Add(path); err != nil {
				stderrLogger.Fatal(err)
			}
		}
		return nil
	})
}

// watch events
func (n nginxWatch) watchEvents() {
	for {
		select {
		case event, ok := <-n.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				if filepath.Base(event.Name) == "..data" {
					stdoutLogger.Println("config map updated")
					nginxPid, nginxPidErr := n.getMasterNginxPid()
					if nginxPidErr != nil {
						stderrLogger.Printf("getting master nginx pid failed: %s", nginxPidErr.Error())
						continue
					}
					if err := n.signalNginxReload(nginxPid); err != nil {
						stderrLogger.Printf("signaling master nginx process failed: %s", err)
					}
				}
			}
		case err, ok := <-n.watcher.Errors:
			if !ok {
				return
			}
			stderrLogger.Printf("received watcher.Error: %s", err)
		}
	}
}

func main() {
	watcher, watcherErr := fsnotify.NewWatcher()
	if watcherErr != nil {
		stderrLogger.Fatal(watcherErr)
	}
	defer watcher.Close()
	done := make(chan bool)
	w := nginxWatch{watcher: watcher}
	go w.watchEvents()
	pathToWatch, ok := os.LookupEnv(watchPathEnvVarName)
	if !ok {
		pathToWatch = defaultNginxConfPath
	}
	w.watchDir(pathToWatch)
	<-done
}
