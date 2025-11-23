package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	statsURL       = "http://srv.msk01.gigacorp.local/_stats"
	loadAvgLimit   = 30.0
	memUsageLimit  = 80
	diskUsageLimit = 90
	netUsageLimit  = 90

	pollInterval   = 5 * time.Second
	errorThreshold = 3
)

func main() {
	var errorCount int

	for {
		if err := pollOnce(); err != nil {
			errorCount++
			if errorCount >= errorThreshold {
				fmt.Println("Unable to fetch server statistic.")
			}
		} else {
			errorCount = 0
		}

		time.Sleep(pollInterval)
	}
}

func pollOnce() error {
	resp, err := http.Get(statsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	raw := strings.TrimSpace(string(bodyBytes))
	parts := strings.Split(raw, ",")
	if len(parts) != 7 {
		return fmt.Errorf("wrong fields count: %d", len(parts))
	}

	values := make([]float64, 7)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return err
		}
		values[i] = v
	}

	// parse numbers
	loadAvg := values[0]

	memTotal := int64(values[1])
	memUsed := int64(values[2])

	diskTotal := int64(values[3])
	diskUsed := int64(values[4])

	netBandwidth := int64(values[5])
	netUsed := int64(values[6])

	if memTotal <= 0 || diskTotal <= 0 || netBandwidth <= 0 {
		return fmt.Errorf("invalid totals")
	}

	// 1. LOAD AVERAGE
	if loadAvg > loadAvgLimit {
		fmt.Printf("Load Average is too high: %.0f\n", loadAvg)
	}

	// 2. MEMORY
	memUsage := int(memUsed * 100 / memTotal)
	if memUsage > memUsageLimit {
		fmt.Printf("Memory usage too high: %d%%\n", memUsage)
	}

	// 3. DISK
	diskUsagePercent := int(diskUsed * 100 / diskTotal)
	if diskUsagePercent > diskUsageLimit {
		freeBytes := diskTotal - diskUsed
		if freeBytes < 0 {
			freeBytes = 0
		}
		freeMb := freeBytes / 1_000_000
		fmt.Printf("Free disk space is too low: %d Mb left\n", freeMb)
	}

	// 4. NETWORK
	netUsagePercent := int(netUsed * 100 / netBandwidth)
	if netUsagePercent > netUsageLimit {
		free := netBandwidth - netUsed
		if free < 0 {
			free = 0
		}
		availableMbit := int(free / 1_000_000)
		fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", availableMbit)
	}

	return nil
}