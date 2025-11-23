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
	statsURL = "http://srv.msk01.gigacorp.local/_stats"

	loadAvgLimit    = 30.0  // > 30
	memUsageLimit   = 80.0  // > 80%
	diskUsageLimit  = 90.0  // > 90%
	netUsageLimit   = 90.0  // > 90%
	mb              = 1024 * 1024
	bitsInByte      = 8
	megaBitDivider  = 1000 * 1000 // перевод в Мбит/с
	pollInterval    = 5 * time.Second
	errorThreshold  = 3
)

func main() {
	var errorCount int

	for {
		if err := pollOnce(&errorCount); err != nil {
			errorCount++
			if errorCount >= errorThreshold {
				fmt.Println("Unable to fetch server statistic.")
			}
		} else {
			// при успешном чтении статистики сбрасываем счётчик ошибок
			errorCount = 0
		}

		time.Sleep(pollInterval)
	}
}

func pollOnce(errorCount *int) error {
	resp, err := http.Get(statsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	body := strings.TrimSpace(string(bodyBytes))
	parts := strings.Split(body, ",")
	if len(parts) != 7 {
		return fmt.Errorf("unexpected number of fields: %d", len(parts))
	}

	values := make([]float64, 7)
	for i, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return fmt.Errorf("parse error at index %d: %w", i, err)
		}
		values[i] = v
	}

	loadAvg := values[0]
	memTotal := values[1]
	memUsed := values[2]
	diskTotal := values[3]
	diskUsed := values[4]
	netBandwidth := values[5]
	netUsed := values[6]

	// базовая валидация, чтобы не делить на ноль
	if memTotal <= 0 || diskTotal <= 0 || netBandwidth <= 0 {
		return fmt.Errorf("invalid totals in stats")
	}

	// 1. Load Average
	if loadAvg > loadAvgLimit {
		fmt.Printf("Load Average is too high: %.0f\n", loadAvg)
	}

	// 2. Memory usage
	memUsage := int(memUsed * 100 / memTotal)
	if memUsage > 80 {
		fmt.Printf("Memory usage too high: %d%%\n", memUsage)
	}
	
	// 3. Disk usage / free Mb
	diskUsagePercent := (diskUsed / diskTotal) * 100.0
	if diskUsagePercent > diskUsageLimit {
		freeBytes := diskTotal - diskUsed
		if freeBytes < 0 {
			freeBytes = 0
		}
		freeMb := int64(freeBytes) / mb
		fmt.Printf("Free disk space is too low: %d Mb left\n", freeMb)
	}

	// 4. Network usage / free Mbit/s
	netUsagePercent := int(netUsed * 100 / netBandwidth)
	if netUsagePercent > 90 {
		free := netBandwidth - netUsed
		availableMbit := int(free / 1_000_000)
		fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", availableMbit)
	}

	return nil
}