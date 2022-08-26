package rclone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	Address string
}

type transferring struct {
	Name string `json:"name"`
}

type responseStats struct {
	Transferring []*transferring `json:"transferring"`
	Error        string          `json:"error"`
}

type bwlimitResponse struct {
	BytesPerSecond int `json:"bytesPerSecond"`
	//Rate string
	Error string `json:"error"`
}

type bwlimitRequest struct {
	//BytesPerSecond int `json:"bytesPerSecond"`
	Rate string
}

func (cl *Client) GetActiveTransferCount() (int, error) {
	resp, err := http.Post(cl.Address+"/core/stats", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return -1, err
	}
	var stats responseStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return -1, err
	}
	if stats.Error != "" {
		return -1, fmt.Errorf("rclone API error: %s", stats.Error)
	}
	return len(stats.Transferring), nil
}

func (cl *Client) GetLimit() (int, error) {
	resp, err := http.Post(cl.Address+"/core/bwlimit", "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return -1, err
	}
	var stats bwlimitResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return -1, err
	}
	if stats.Error != "" {
		return -1, fmt.Errorf("rclone API error: %s", stats.Error)
	}
	return stats.BytesPerSecond, nil
}

func (cl *Client) SetLimit(bytesPerSecond int) error {
	v := url.Values{}
	// NOTE: for some reason the API only seems to accept human readable parameters, so we convert to KiB/s
	v.Set("rate", fmt.Sprintf("%dK", bytesPerSecond/1024))

	resp, err := http.PostForm(cl.Address+"/core/bwlimit", v)
	if err != nil {
		return err
	}
	var stats bwlimitResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return err
	}
	if stats.Error != "" {
		return fmt.Errorf("rclone API error: %s", stats.Error)
	}
	if stats.BytesPerSecond != bytesPerSecond {
		return fmt.Errorf("rclone failed to accept new bwlimit. got %d, want %d", stats.BytesPerSecond, bytesPerSecond)
	}
	return nil
}
