package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

func (a *Agent) Run(ctx context.Context) error {
	if err := a.initialize(); err != nil {
		a.closeDB()
		_ = a.setState(StateStopped)
		return err
	}
	if err := a.registerUntilSuccess(ctx); err != nil {
		a.closeDB()
		_ = a.setState(StateStopped)
		return err
	}
	_ = a.setState(StateRunning)
	if err := a.flushPending(ctx); err != nil {
		log.Printf("flush pending skipped: %v", err)
	}

	serverDone := make(chan struct{})
	go a.startLocalServer(serverDone)

	loopCancel, loopsDone := a.startLoops(ctx)

	for {
		select {
		case <-ctx.Done():
			a.requestShutdown("context canceled")
		case <-a.reauthCh:
			if a.getState() == StateDraining || a.getState() == StateStopped {
				continue
			}
			_ = a.setState(StateAuthExpired)
			loopCancel()
			<-loopsDone
			if err := a.registerUntilSuccess(ctx); err != nil {
				_ = a.setState(StateStopped)
				return err
			}
			if err := a.flushPending(ctx); err != nil {
				log.Printf("flush pending failed after reauth: %v", err)
			}
			_ = a.setState(StateRunning)
			loopCancel, loopsDone = a.startLoops(ctx)
		case reason := <-a.shutdownCh:
			log.Printf("agent draining: %s", reason)
			_ = a.setState(StateDraining)
			loopCancel()
			<-loopsDone
			a.waitRunningTasks(30 * time.Second)
			if err := a.flushPending(ctx); err != nil {
				log.Printf("final flush pending failed: %v", err)
			}
			a.closeDB()
			_ = a.setState(StateStopped)
			close(serverDone)
			return nil
		}
	}
}

func (a *Agent) closeDB() {
	if a.db == nil {
		return
	}
	if err := a.db.Close(); err != nil {
		log.Printf("close sqlite failed: %v", err)
	}
	a.db = nil
}

func (a *Agent) startLocalServer(done <-chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service":   "luoyi-agent",
			"status":    "ok",
			"timestamp": time.Now().Unix(),
			"agent_id":  a.agentID,
			"state":     string(a.getState()),
		})
	})
	if a.cfg.MetricsEnabled && a.obs != nil {
		metricsPath := a.cfg.MetricsPath
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		mux.Handle(metricsPath, a.obs.Handler())
	}

	srv := &http.Server{Addr: a.cfg.LocalAddr, Handler: mux}
	go func() {
		<-done
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("luoyi-agent local endpoint listening on %s", a.cfg.LocalAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("agent local endpoint stopped: %v", err)
	}
}

func (a *Agent) startLoops(parent context.Context) (context.CancelFunc, <-chan struct{}) {
	loopCtx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.heartbeatLoop(loopCtx)
	}()
	go func() {
		defer wg.Done()
		a.pollLoop(loopCtx)
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return cancel, done
}
