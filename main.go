package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
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
	var showVersion, showConfig, localMode bool
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showConfig, "x", false, "show config")
	flag.BoolVar(&localMode, "local", false, "local mode (for development)")
	logLevel := flag.String("log-level", "info", "log level (trace, debug, info, warn, error)")
	flag.VisitAll(overrideWithEnv)
	flag.Parse()

	if showVersion {
		fmt.Printf("mirage %v (%v)\n", version, buildDate)
		return
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"trace", "debug", "info", "warn", "error"},
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Printf("[debug] setting log level: %s", *logLevel)

	cfg := NewConfig(*confFile)
	if showConfig {
		fmt.Println("mirage config:")
		pp.Print(cfg)
		fmt.Println("") // add linebreak
	}
	cfg.localMode = localMode
	Setup(cfg)
	Run()
}

func overrideWithEnv(f *flag.Flag) {
	name := strings.ToUpper(f.Name)
	name = strings.Replace(name, "-", "_", -1)
	if s := os.Getenv("MIRAGE_" + name); s != "" {
		f.Value.Set(s)
	}
}
