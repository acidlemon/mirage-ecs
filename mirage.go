package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"golang.org/x/net/context"
)

var app *Mirage

type Mirage struct {
	Config       *Config
	WebApi       *WebApi
	ReverseProxy *ReverseProxy
	ECS          ECSInterface
	Route53      *Route53
	CloudWatch   *cloudwatch.CloudWatch

	mu sync.Mutex
}

func Setup(cfg *Config) {
	m := &Mirage{
		Config:       cfg,
		WebApi:       NewWebApi(cfg),
		ReverseProxy: NewReverseProxy(cfg),
		ECS:          NewECS(cfg),
		Route53:      NewRoute53(cfg),
		CloudWatch:   cloudwatch.New(cfg.session),
	}

	app = m
}

func Run() {
	// launch server
	var wg sync.WaitGroup
	for _, v := range app.Config.Listen.HTTP {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			laddr := fmt.Sprintf("%s:%d", app.Config.Listen.ForeignAddress, port)
			listener, err := net.Listen("tcp", laddr)
			if err != nil {
				log.Printf("[error] cannot listen %s: %s", laddr, err)
				return
			}

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				app.ServeHTTPWithPort(w, req, port)
			})

			log.Println("[info] listen port:", port)
			http.Serve(listener, mux)
		}(v.ListenPort)
	}
	app.ECS.Run()
	go app.RunAccessCountCollector()
	log.Println("[info] Launch succeeded!")

	wg.Wait()
}

func (m *Mirage) ServeHTTPWithPort(w http.ResponseWriter, req *http.Request, port int) {
	host := strings.ToLower(strings.Split(req.Host, ":")[0])

	switch {
	case m.isWebApiHost(host):
		m.WebApi.ServeHTTP(w, req)

	case m.isDockerHost(host):
		m.ReverseProxy.ServeHTTPWithPort(w, req, port)

	default:
		if req.URL.Path == "/" {
			// otherwise root returns 200 (for healthcheck)
			http.Error(w, "mirage-ecs", http.StatusOK)
		} else {
			// return 404
			log.Printf("[warn] host %s is not found", host)
			http.NotFound(w, req)
		}
	}

}

func (m *Mirage) isDockerHost(host string) bool {
	if strings.HasSuffix(host, m.Config.Host.ReverseProxySuffix) {
		subdomain := strings.ToLower(strings.Split(host, ".")[0])
		return m.ReverseProxy.Exists(subdomain)
	}

	return false
}

func (m *Mirage) isWebApiHost(host string) bool {
	return isSameHost(m.Config.Host.WebApi, host)
}

func isSameHost(s1 string, s2 string) bool {
	lower1 := strings.Trim(strings.ToLower(s1), " ")
	lower2 := strings.Trim(strings.ToLower(s2), " ")

	return lower1 == lower2
}

func (m *Mirage) RunAccessCountCollector() {
	tk := time.NewTicker(m.ReverseProxy.accessCounterUnit)
	for range tk.C {
		all := m.ReverseProxy.CollectAccessCounters()
		s, _ := json.Marshal(all)
		log.Printf("[info] access counters: %s", string(s))
		if m.Config.localMode {
			log.Println("[info] local mode: skip put metrics")
		} else {
			m.PutAllMetrics(all)
		}
	}
}

const (
	CloudWatchMetricNameSpace = "mirage-ecs"
	CloudWatchMetricName      = "RequestCount"
	CloudWatchDimensionName   = "subdomain"
)

func (m *Mirage) GetAccessCount(subdomain string, duration time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := m.CloudWatch.GetMetricDataWithContext(ctx, &cloudwatch.GetMetricDataInput{
		StartTime: aws.Time(time.Now().Add(-duration)),
		EndTime:   aws.Time(time.Now()),
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id: aws.String("request_count"),
				MetricStat: &cloudwatch.MetricStat{
					Metric: &cloudwatch.Metric{
						Dimensions: []*cloudwatch.Dimension{
							{
								Name:  aws.String(CloudWatchDimensionName),
								Value: aws.String(subdomain),
							},
						},
						MetricName: aws.String(CloudWatchMetricName),
						Namespace:  aws.String(CloudWatchMetricNameSpace),
					},
					Period: aws.Int64(int64(duration.Seconds())),
					Stat:   aws.String("Sum"),
				},
			},
		},
	})
	if err != nil {
		return 0, err
	}
	var sum int64
	for _, v := range res.MetricDataResults {
		for _, vv := range v.Values {
			sum += int64(aws.Float64Value(vv))
		}
	}
	return sum, nil
}

func (m *Mirage) PutAllMetrics(all map[string]map[time.Time]int64) {
	pmInput := cloudwatch.PutMetricDataInput{
		Namespace: aws.String(CloudWatchMetricNameSpace),
	}
	for subdomain, counters := range all {
		for ts, count := range counters {
			log.Printf("[debug] access for %s %s %d", subdomain, ts.Format(time.RFC3339), count)
			pmInput.MetricData = append(pmInput.MetricData, &cloudwatch.MetricDatum{
				MetricName: aws.String(CloudWatchMetricName),
				Timestamp:  aws.Time(ts),
				Value:      aws.Float64(float64(count)),
				Dimensions: []*cloudwatch.Dimension{
					{
						Name:  aws.String(CloudWatchDimensionName),
						Value: aws.String(subdomain),
					},
				},
			})
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if len(pmInput.MetricData) > 0 {
		_, err := m.CloudWatch.PutMetricDataWithContext(ctx, &pmInput)
		if err != nil {
			log.Printf("[error] %s", err)
		}
	}
}

func (app *Mirage) TryLock() bool {
	return app.mu.TryLock()
}

func (app *Mirage) Unlock() {
	app.mu.Unlock()
}
