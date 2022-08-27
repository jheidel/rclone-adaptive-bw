package main

import (
	"context"
	"flag"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"

	"go.einride.tech/pid"

	"rclone-adaptive-bw/ping"
	"rclone-adaptive-bw/rclone"
)

var (
	rcloneRc = flag.String("rclone_rc", "http://localhost:5572", "Address to Rclone remote control API")
)

func main() {
	flag.Parse()

	// Configure logging.
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

	log.Info("Starting")

	rc := &rclone.Client{
		Address: *rcloneRc,
	}

	count, err := rc.GetActiveTransferCount()
	if err != nil {
		log.Fatalf("%v", err)
	}

	log.Infof("Currently have %d active txfrs", count)

	currentLimit, err := rc.GetLimit()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Infof("Current limit is %d", currentLimit)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	p, err := ping.New(ctx, &wg, &ping.Options{
		Hosts:   []string{"1.1.1.1", "8.8.8.8"},
		History: 30 * time.Second,
	})
	if err != nil {
		log.Fatalf("%v", err)
	}

	ctrl := &pid.Controller{
		Config: pid.ControllerConfig{
			ProportionalGain: 0.25,
			IntegralGain:     0.0,
			DerivativeGain:   0.0,
		},
	}

	setpoint := 70 * time.Millisecond

	target := currentLimit

	last := time.Now()
	delay := 5 * time.Second
	for ctx.Err() == nil {
		if err := rc.SetLimit(target); err != nil {
			log.Fatalf("%v", err)
		}
		time.Sleep(delay)
		delay = 5 * time.Second

		txc, err := rc.GetActiveTransferCount()
		if err != nil {
			log.Fatalf("%v", err)
		}
		if txc == 0 {
			p.Wipe()
			continue
		}

		if !p.IsValid() {
			continue
		}

		s, err := p.Stats()
		if err != nil {
			log.Fatalf("%v", err)
		}
		log.Infof("Stats: %+v", s)

		tailThresh := 120 * time.Millisecond

		if s.Tail > 500*time.Millisecond {
			log.Warnf("Tail is %v, dropping to minimum", s.Tail)
			target = 400 * 1024
			delay = 5 * time.Second
			p.Wipe()
			ctrl.Reset()
			continue
		}

		if s.Tail > tailThresh*2 {
			// Something isn't working right, drop back to mins
			target = target * 3 / 4
			log.Warnf("Tail is %v, cutting bandwidth", s.Tail)
			delay = 5 * time.Second
			p.Wipe()
			ctrl.Reset()
			continue
		}

		now := time.Now()
		delta := now.Sub(last)
		last = now

		sig := float64(s.Mean)
		if s.Tail > tailThresh {
			// Penalize bad tail latencies
			sig += float64(s.Tail-tailThresh) / 10
		}

		ctrl.Update(pid.ControllerInput{
			ReferenceSignal:  float64(setpoint),
			ActualSignal:     sig,
			SamplingInterval: delta,
		})

		o := ctrl.State.ControlSignal

		// Loose conversion of controller signal into increase in bandwidth
		oc := int(o / 30e6 * 300 * 1024)

		target += oc

		lowerBound := 350 * 1024
		upperBound := 5 * 1024 * 1024
		if target < lowerBound {
			target = lowerBound
		}
		if target > upperBound {
			target = upperBound
		}

		log.Infof("Control output: %v (%d KiB/s), target is %d KiB/s", time.Duration(o), oc/1024, target/1024)
	}

	cancel()
	wg.Wait()
}
