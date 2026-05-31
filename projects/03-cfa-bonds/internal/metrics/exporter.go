package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Exporter struct {
	srv *http.Server
	log *slog.Logger
}

func NewExporter(addr string, log *slog.Logger, collectors ...prometheus.Collector) *Exporter {
	reg := prometheus.NewRegistry()
	reg.MustRegister(SettlementDuration, SettlementOutcomes)
	for _, c := range collectors {
		if c != nil {
			reg.MustRegister(c)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	return &Exporter{
		srv: &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second},
		log: log,
	}
}

func (e *Exporter) Start() error {
	e.log.Info("metrics exporter listening", "addr", e.srv.Addr)
	if err := e.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("metrics server: %w", err)
	}
	return nil
}

func (e *Exporter) Shutdown(ctx context.Context) error {
	return e.srv.Shutdown(ctx)
}
