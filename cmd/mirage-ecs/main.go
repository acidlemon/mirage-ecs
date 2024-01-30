package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
	"gopkg.in/yaml.v2"
)

var (
	Version   string
	buildDate string
)

func main() {
	confFile := flag.String("conf", "", "specify config file or S3 URL")
	domain := flag.String("domain", ".local", "reverse proxy suffix")
	var showVersion, showConfig, localMode, compatV1 bool
	var defaultPort int
	var logFormat, logLevel string
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showConfig, "x", false, "show config")
	flag.BoolVar(&localMode, "local", false, "local mode (for development)")
	flag.BoolVar(&compatV1, "compat-v1", false, "compatibility mode for v1")
	flag.IntVar(&defaultPort, "default-port", 80, "default port number")
	flag.StringVar(&logFormat, "log-format", "text", "log format (text, json)")
	flag.StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	flag.VisitAll(overrideWithEnv)
	flag.Parse()

	mirageecs.SetLogLevel(logLevel)

	if showVersion {
		fmt.Printf("mirage-ecs %s (%s)\n", Version, buildDate)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := mirageecs.NewConfig(ctx, &mirageecs.ConfigParams{
		Path:        *confFile,
		LocalMode:   localMode,
		Domain:      *domain,
		DefaultPort: defaultPort,
		CompatV1:    compatV1,
		LogFormat:   logFormat,
	})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if showConfig {
		yaml.NewEncoder(os.Stdout).Encode(cfg)
		return
	}
	mirageecs.Version = Version
	app := mirageecs.New(ctx, cfg)
	if err := app.Run(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func overrideWithEnv(f *flag.Flag) {
	name := strings.ToUpper(f.Name)
	name = strings.Replace(name, "-", "_", -1)
	if s := os.Getenv("MIRAGE_" + name); s != "" {
		f.Value.Set(s)
	}
}
