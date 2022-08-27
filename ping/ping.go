package ping

import (
	"context"
	"github.com/go-ping/ping"
	"github.com/montanaflynn/stats"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type pingResp struct {
	t time.Time
	d time.Duration
}

type Pinger struct {
	history []*pingResp
	l       sync.Mutex

	config *Options
}

type Stats struct {
	Mean time.Duration
	Tail time.Duration
}

type Options struct {
	Hosts   []string
	History time.Duration
}

func (p *Pinger) wipe(now time.Time, d time.Duration) {
	// Filter out any events that have expired
	var history []*pingResp
	for _, h := range p.history {
		if now.Sub(h.t) <= d {
			history = append(history, h)
		}
	}
	p.history = history
}

func (p *Pinger) Wipe() {
	p.l.Lock()
	defer p.l.Unlock()
	p.history = nil
}

func (p *Pinger) IsValid() bool {
	p.l.Lock()
	defer p.l.Unlock()
	return len(p.history) > 0
}

func (p *Pinger) report(d time.Duration) {
	p.l.Lock()
	defer p.l.Unlock()

	now := time.Now()

	p.history = append(p.history, &pingResp{t: now, d: d})
	p.wipe(now, p.config.History)
}

func (p *Pinger) Stats() (*Stats, error) {
	p.l.Lock()
	data := []float64{}
	for _, h := range p.history {
		data = append(data, float64(h.d))
	}
	p.l.Unlock()

	if len(data) == 0 {
		return &Stats{}, nil
	}

	mean, err := stats.Mean(data)
	if err != nil {
		return nil, err
	}

	tail, err := stats.Percentile(data, 90)
	if err != nil {
		return nil, err
	}

	return &Stats{
		Mean: time.Duration(mean),
		Tail: time.Duration(tail),
	}, nil
}

func New(ctx context.Context, wg *sync.WaitGroup, config *Options) (*Pinger, error) {
	p := &Pinger{
		config: config,
	}
	var pingers []*ping.Pinger
	for _, h := range config.Hosts {
		pg, err := ping.NewPinger(h)
		if err != nil {
			return nil, err
		}
		pingers = append(pingers, pg)
	}
	for i, pinger := range pingers {
		pinger.OnRecv = func(pkt *ping.Packet) {
			log.Infof("Ping response from %q: %v", pkt.Addr, pkt.Rtt)
			p.report(pkt.Rtt)
		}
		go func(pg *ping.Pinger, delay time.Duration) {
			wg.Add(1)
			defer wg.Done()
			time.Sleep(delay)
			go pg.Run()
			<-ctx.Done()
			pg.Stop()
		}(pinger, time.Millisecond*time.Duration(1000/len(pingers)*i))
	}
	return p, nil
}
