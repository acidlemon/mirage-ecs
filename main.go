package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/hashicorp/logutils"
	"github.com/k0kubun/pp"
)

var (
	version   string
	buildDate string
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	confFile := flag.String("conf", "config.yml", "specify config file")
	var showVersion, showConfig bool
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showConfig, "x", false, "show config")
	logLevel := flag.String("log-level", "info", "log level (trace, debug, info, warn, error)")
	flag.Parse()

	if showVersion {
		fmt.Printf("mirage %v (%v)\n", version, buildDate)
		return
	}

	cfg := NewConfig(*confFile)

	if showConfig {
		fmt.Println("mirage config:")
		pp.Print(cfg)
		fmt.Println("") // add linebreak
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	Setup(cfg)
	Run()
}
