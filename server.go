package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kardianos/service"
	"github.com/topxeq/quicksharex/server/apis"
	"github.com/topxeq/quicksharex/server/libs/cfg"
	"github.com/topxeq/tk"
)

var basePathG string
var logFileG string

func logWithTime(formatA string, argsA ...interface{}) {
	if logFileG == "" {
		return
	}

	tk.AppendStringToFile(fmt.Sprintf(fmt.Sprintf("[%v] ", time.Now())+formatA+"\n", argsA...), logFileG)
}

type program struct {
	BasePath string
}

func (p *program) Start(s service.Service) error {
	go p.run()

	return nil
}

func (p *program) run() {
	go doWork()
}

func (p *program) Stop(s service.Service) error {
	return nil
}

var exit = make(chan struct{})

func Svc() {

	if basePathG == "" {
		if strings.HasPrefix(runtime.GOOS, "win") {
			basePathG = "c:\\quicksharex"
		} else {
			basePathG = "/quicksharex"
		}

		tk.EnsureMakeDirs(basePathG)
	}

	logFileG = filepath.Join(basePathG, "quicksharex.log")

	defer func() {
		if v := recover(); v != nil {
			logWithTime("panic in run %v", v)
		}
	}()

	logWithTime("quicksharex at: %v", tk.GetNowTimeString())

	config := cfg.NewConfigFrom(filepath.Join(basePathG, "config.json"))
	srvShare := apis.NewSrvShare(config)

	// TODO: using httprouter instead
	mux := http.NewServeMux()
	mux.HandleFunc(config.PathLogin, srvShare.LoginHandler)
	mux.HandleFunc(config.PathStartUpload, srvShare.StartUploadHandler)
	mux.HandleFunc(config.PathUpload, srvShare.UploadHandler)
	mux.HandleFunc(config.PathFinishUpload, srvShare.FinishUploadHandler)
	mux.HandleFunc(config.PathDownload, srvShare.DownloadHandler)
	mux.HandleFunc(config.PathFileInfo, srvShare.FileInfoHandler)
	mux.HandleFunc(config.PathClient, srvShare.ClientHandler)

	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", config.HostName, config.Port),
		Handler:        mux,
		MaxHeaderBytes: config.MaxHeaderBytes,
		ReadTimeout:    time.Duration(config.ReadTimeout) * time.Millisecond,
		WriteTimeout:   time.Duration(config.WriteTimeout) * time.Millisecond,
		IdleTimeout:    time.Duration(config.IdleTimeout) * time.Millisecond,
	}

	logWithTime("quickshare starts @ %s:%d, public path: %v", config.HostName, config.Port, config.PathPublic)
	// err := open.Start(fmt.Sprintf("http://%s:%d", config.HostName, config.Port))
	// if err != nil {
	// 	log.Println(err)
	// }
	log.Fatal(server.ListenAndServe())

}

func doWork() {

	go Svc()

	for {
		select {
		case <-exit:
			os.Exit(0)
			return
		}
	}
}

func stopWork() {
	exit <- struct{}{}
}

func initSvc() *service.Service {
	svcConfigT := &service.Config{
		Name:        "quicksharexSvc",
		DisplayName: "quicksharexSvc",
		Description: "quicksharexSvc",
	}

	prgT := &program{BasePath: basePathG}
	var s, err = service.New(prgT, svcConfigT)

	if err != nil {
		logWithTime("%s unable to start: %s\n", svcConfigT.DisplayName, err)
		return nil
	}

	return &s
}

func runCmd(cmdLineA []string) {
	cmdT := ""

	for _, v := range cmdLineA {
		if !strings.HasPrefix(v, "-") {
			cmdT = v
			break
		}
	}

	basePathG = tk.GetSwitchWithDefaultValue(cmdLineA, "-base=", basePathG)

	tk.EnsureMakeDirs(basePathG)

	if !tk.IfFileExists(basePathG) {
		fmt.Printf("base path not exists: %v, use current directory instead\n", basePathG)
		basePathG = "."
	}

	if !tk.IsDirectory(basePathG) {
		fmt.Printf("base path not exists: %v\n", basePathG)
		return
	}

	switch cmdT {
	case "", "run":
		s := initSvc()

		if s == nil {
			logWithTime("Failed to init service")
			break
		}

		err := (*s).Run()
		if err != nil {
			logWithTime("Service \"%s\" failed to run.", (*s).String())
		}
	case "installonly":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}

		err := (*s).Install()
		if err != nil {
			fmt.Printf("Failed to install: %s\n", err)
			return
		}

		fmt.Printf("Service \"%s\" installed.\n", (*s).String())

	case "install":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}

		fmt.Printf("Installing service \"%v\"...\n", (*s).String())

		err := (*s).Install()
		if err != nil {
			fmt.Printf("Failed to install: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" installed.\n", (*s).String())

		fmt.Printf("Starting service \"%v\"...\n", (*s).String())

		err = (*s).Start()
		if err != nil {
			fmt.Printf("Failed to start: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" started.\n", (*s).String())
	case "uninstall":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}

		err := (*s).Stop()
		if err != nil {
			fmt.Printf("Failed to stop: %s\n", err)
		} else {
			fmt.Printf("Service \"%s\" stopped.\n", (*s).String())
		}

		err = (*s).Uninstall()
		if err != nil {
			fmt.Printf("Failed to remove: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" removed.\n", (*s).String())
	case "reinstall":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}

		err := (*s).Stop()
		if err != nil {
			fmt.Printf("Failed to stop: %s\n", err)
		} else {
			fmt.Printf("Service \"%s\" stopped.\n", (*s).String())
		}

		err = (*s).Uninstall()
		if err != nil {
			fmt.Printf("Failed to remove: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" removed.\n", (*s).String())

		err = (*s).Install()
		if err != nil {
			fmt.Printf("Failed to install: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" installed.\n", (*s).String())

		err = (*s).Start()
		if err != nil {
			fmt.Printf("Failed to start: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" started.\n", (*s).String())
	case "start":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}

		err := (*s).Start()
		if err != nil {
			fmt.Printf("Failed to start: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" started.\n", (*s).String())
	case "stop":
		s := initSvc()

		if s == nil {
			fmt.Printf("Failed to install")
			break
		}
		err := (*s).Stop()
		if err != nil {
			fmt.Printf("Failed to stop: %s\n", err)
			return
		}
		fmt.Printf("Service \"%s\" stopped.\n", (*s).String())
	default:
		fmt.Println("unknown command")
		break
	}

}

func main() {

	if strings.HasPrefix(runtime.GOOS, "win") {
		basePathG = "c:\\quicksharex"
	} else {
		basePathG = "/quicksharex"
	}

	logFileG = filepath.Join(basePathG, "quicksharex.log")

	tk.Pl("Args count: %v", len(os.Args))

	if len(os.Args) < 2 {

		s := initSvc()

		if s == nil {
			logWithTime("Failed to init service")
			return
		}

		err := (*s).Run()
		if err != nil {
			logWithTime("Service \"%s\" failed to run.", (*s).String())
		}

		return
	}

	runCmd(os.Args[1:])

}
