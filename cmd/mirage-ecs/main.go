package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	mirageecs "github.com/acidlemon/mirage-ecs"
	"github.com/hashicorp/logutils"
	"gopkg.in/yaml.v2"
)

var (
	Version   string
	buildDate string
)

func main() {
	confFile := flag.String("conf", "", "specify config file or S3 URL")
	domain := flag.String("domain", ".local", "reverse proxy suffix")
	var showVersion, showConfig, localMode bool
	var defaultPort int
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showConfig, "x", false, "show config")
	flag.BoolVar(&localMode, "local", false, "local mode (for development)")
	flag.IntVar(&defaultPort, "default-port", 80, "default port number")
	logLevel := flag.String("log-level", "info", "log level (trace, debug, info, warn, error)")
	flag.VisitAll(overrideWithEnv)
	flag.Parse()

	if showVersion {
		fmt.Printf("mirage-ecs %s (%s)\n", Version, buildDate)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := mirageecs.NewConfig(&mirageecs.ConfigParams{
		Path:        *confFile,
		LocalMode:   localMode,
		Domain:      *domain,
		DefaultPort: defaultPort,
	})
	if err != nil {
		log.Fatalf("[error] %s", err)
	}
	if showConfig {
		yaml.NewEncoder(os.Stdout).Encode(cfg)
		return
	}
	log.Println("[info] mirage-ecs version:", Version)
	app := mirageecs.New(cfg)
	app.Run(ctx)
}

func overrideWithEnv(f *flag.Flag) {
	name := strings.ToUpper(f.Name)
	name = strings.Replace(name, "-", "_", -1)
	if s := os.Getenv("MIRAGE_" + name); s != "" {
		f.Value.Set(s)
	}
}
