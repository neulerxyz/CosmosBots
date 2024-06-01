package nodebot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/neulerxyz/CosmosBots/config"
)

type NodeBot struct {
	cfg     *config.Config
	alertCh chan string
}

type NodeStatus struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
	} `json:"result"`
}

func NewNodeBot(cfg *config.Config, alertCh chan string) *NodeBot {
	return &NodeBot{cfg: cfg, alertCh: alertCh}
}

func (nb *NodeBot) Start() {
	go func() {
		for {
			nb.checkStatuses()
			time.Sleep(10 * time.Second) // Adjust interval as needed
		}
	}()
}

func (nb *NodeBot) checkStatuses() {
	referenceHeight, err := nb.getLatestBlockHeight(nb.cfg.GetReferenceEndpoint())
	if err != nil {
		log.Printf("Error getting reference block height: %v", err)
		return
	}

	for _, url := range nb.cfg.GetCheckEndpoints() {
		status, err := nb.getNodeStatus(url)
		if err != nil {
			log.Printf("Error getting node status for %s: %v", url, err)
			continue
		}
		latestBlockHeight, err := strconv.Atoi(status.Result.SyncInfo.LatestBlockHeight)
		if err != nil {
			log.Printf("Error parsing block height for %s: %v", url, err)
			continue
		}

		if latestBlockHeight-referenceHeight > 1 {
			message := fmt.Sprintf("Alert: Node %s is behind by more than 1 blocks. Latest block height: %d, Reference block height: %d", url, latestBlockHeight, referenceHeight)
			nb.alertCh <- message
		}

		if status.Result.SyncInfo.CatchingUp {
			message := fmt.Sprintf("Alert: Node %s is catching up.", url)
			nb.alertCh <- message
		}
	}
}

func (nb *NodeBot) getLatestBlockHeight(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var status NodeStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		return 0, err
	}

	height, err := strconv.Atoi(status.Result.SyncInfo.LatestBlockHeight)
	if err != nil {
		return 0, err
	}
	return height, nil
}

func (nb *NodeBot) getNodeStatus(url string) (*NodeStatus, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var status NodeStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
