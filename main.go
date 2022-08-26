package main

import (
	"flag"
	log "github.com/sirupsen/logrus"

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

	limit, err := rc.GetLimit()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Infof("Current limit is %d", limit)

	if err := rc.SetLimit(400 * 1024); err != nil {
		log.Fatalf("%v", err)
	}
}
