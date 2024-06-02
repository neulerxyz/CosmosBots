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
	cfg               *config.Config
	alertCh           chan string
	previousHeights   map[string]int
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
	return &NodeBot{
		cfg:             cfg,
		alertCh:         alertCh,
		previousHeights: make(map[string]int),
	}
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
	referenceHeight, referenceURL, err := nb.getLatestBlockHeight(nb.cfg.GetReferenceEndpoints())
	if err != nil {
		log.Printf("Error getting reference block height: %v", err)
		return
	}

	nb.checkAndUpdateHeight(referenceURL, referenceHeight)

	for _, url := range nb.cfg.GetCheckEndpoints() {
		if nb.cfg.FaultyCheckEndpoints[url] {
			continue // Skip already marked faulty endpoints
		}

		status, err := nb.getNodeStatus(url + "/status")
		if err != nil {
			log.Printf("Error getting node status for %s: %v", url, err)
			nb.cfg.AddFaultyCheckEndpoint(url)
			continue
		}
		latestBlockHeight, err := strconv.Atoi(status.Result.SyncInfo.LatestBlockHeight)
		if err != nil {
			log.Printf("Error parsing block height for %s: %v", url, err)
			nb.cfg.AddFaultyCheckEndpoint(url)
			continue
		}

		isSynced := !status.Result.SyncInfo.CatchingUp
		nb.cfg.SetNodeStatus(url, config.NodeStatus{
			LatestBlockHeight: latestBlockHeight,
			IsSynced:          isSynced,
		})

		nb.checkAndUpdateHeight(url, latestBlockHeight)
		if latestBlockHeight > referenceHeight {
			log.Printf("Warn: Reference %s is behind by %d blocks.", referenceURL, latestBlockHeight-referenceHeight)
			continue
		} else if latestBlockHeight-referenceHeight > 50 {
			message := fmt.Sprintf("Alert: Node %s is behind by more than 50 blocks. Latest block height: %d, Reference block height: %d", url, latestBlockHeight, referenceHeight)
			nb.alertCh <- message
		}

		if status.Result.SyncInfo.CatchingUp {
			message := fmt.Sprintf("Alert: Node %s is catching up.", url)
			nb.alertCh <- message
		}
	}
}

func (nb *NodeBot) checkAndUpdateHeight(url string, currentHeight int) {
	previousHeight, exists := nb.previousHeights[url]
	if !exists {
		nb.previousHeights[url] = currentHeight
		return
	}

	if currentHeight == previousHeight {
		isReference := false
		for _, refURL := range nb.cfg.GetReferenceEndpoints() {
			if url == refURL {
				isReference = true
				break
			}
		}
		if isReference {
			message := fmt.Sprintf("Alert: Reference node %s is stuck.", url)
			nb.cfg.AddFaultyReferenceEndpoint(url)
			nb.alertCh <- message
			nb.removeReferenceEndpoint(url) // Remove from reference endpoints
		} else {
			message := fmt.Sprintf("Alert: Node %s is stuck.", url)
			nb.cfg.AddFaultyCheckEndpoint(url)
			nb.alertCh <- message
		}
	} else {
		nb.previousHeights[url] = currentHeight
	}
}

func (nb *NodeBot) removeReferenceEndpoint(endpoint string) {
	nb.cfg.Mutex.Lock()
	defer nb.cfg.Mutex.Unlock()
	for i, url := range nb.cfg.ReferenceEndpoints {
		if url == endpoint {
			nb.cfg.ReferenceEndpoints = append(nb.cfg.ReferenceEndpoints[:i], nb.cfg.ReferenceEndpoints[i+1:]...)
			break
		}
	}
}

func (nb *NodeBot) getLatestBlockHeight(urls []string) (int, string, error) {
	for _, url := range urls {
		resp, err := http.Get(url + "/status")
		if err != nil {
			log.Printf("Error getting status from reference endpoint %s: %v", url, err)
			nb.cfg.AddFaultyReferenceEndpoint(url)
			continue
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading body from reference endpoint %s: %v", url, err)
			nb.cfg.AddFaultyReferenceEndpoint(url)
			continue
		}

		var status NodeStatus
		err = json.Unmarshal(body, &status)
		if err != nil {
			log.Printf("Error unmarshalling JSON from reference endpoint %s: %v", url, err)
			nb.cfg.AddFaultyReferenceEndpoint(url)
			continue
		}

		height, err := strconv.Atoi(status.Result.SyncInfo.LatestBlockHeight)
		if err != nil {
			log.Printf("Error converting block height from reference endpoint %s: %v", url, err)
			nb.cfg.AddFaultyReferenceEndpoint(url)
			continue
		}

		// If we get a valid height, return it
		return height, url, nil
	}

	// If no valid reference endpoint was found
	message := "No available reference endpoints found. Cannot check NodeStatus."
	nb.alertCh <- message
	return 0, "", fmt.Errorf("no available reference endpoints")
}

func (nb *NodeBot) getNodeStatus(url string) (*NodeStatus, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error making HTTP request to %s: %v", url, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Non-200 response from %s: %d", url, resp.StatusCode)
		return nil, fmt.Errorf("non-200 response from %s: %d", url, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body from %s: %v", url, err)
		return nil, err
	}

	var status NodeStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		log.Printf("Error unmarshalling JSON from %s: %v", url, err)
		return nil, err
	}

	return &status, nil
}
