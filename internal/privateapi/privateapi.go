package privateapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Ticker struct {
	Symbol             string `json:"symbol"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice          string `json:"lastPrice"`
	Volume             string `json:"volume"`
	MaxDump            string
	MaxPump            string
}

var spotAPIEndpoint string
var futuresAPIEndpoint string

func init() {
	spotAPIEndpoint = "https://api.binance.com"
	futuresAPIEndpoint = "https://fapi.binance.com"
}

func FetchTopGainers(market, timeFrame string) ([]Ticker, error) {
	tickers, err := fetchTickers(market, timeFrame)
	if err != nil {
		return nil, err
	}

	sort.Slice(tickers, func(i, j int) bool {
		p1, _ := strconv.ParseFloat(tickers[i].PriceChangePercent, 64)
		p2, _ := strconv.ParseFloat(tickers[j].PriceChangePercent, 64)
		return p1 > p2
	})

	if len(tickers) > 5 {
		return tickers[:5], nil
	}
	return tickers, nil
}

func FetchPumpDump(market, timeFrame string) ([]Ticker, error) {
	tickers, err := fetchTickers(market, timeFrame)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var filteredTickers []Ticker

	for i := range tickers {
		wg.Add(1)
		go func(t *Ticker) {
			defer wg.Done()
			changePercent, err := strconv.ParseFloat(t.PriceChangePercent, 64)
			if err != nil {
				return
			}
			if abs(changePercent) >= 0.1 {
				maxDump, maxPump, err := calculateMaxDumpPump(market, t.Symbol)
				if err != nil {
					log.Printf("Ошибка при вычислении Max Dump/Pump для %s: %v", t.Symbol, err)
					return
				}
				t.MaxDump = fmt.Sprintf("%.2f", maxDump)
				t.MaxPump = fmt.Sprintf("%.2f", maxPump)

				mu.Lock()
				filteredTickers = append(filteredTickers, *t)
				mu.Unlock()
			}
		}(&tickers[i])
	}

	wg.Wait()

	sort.Slice(filteredTickers, func(i, j int) bool {
		p1, _ := strconv.ParseFloat(filteredTickers[i].PriceChangePercent, 64)
		p2, _ := strconv.ParseFloat(filteredTickers[j].PriceChangePercent, 64)
		return abs(p1) > abs(p2)
	})

	return filteredTickers, nil
}

func fetchTickers(market, timeFrame string) ([]Ticker, error) {
	var baseURL string
	var endpoint string

	if market == "spot" {
		baseURL = spotAPIEndpoint
		endpoint = "/api/v3/ticker/24hr"
	} else if market == "futures" {
		baseURL = futuresAPIEndpoint
		endpoint = "/fapi/v1/ticker/24hr"
	} else {
		return nil, fmt.Errorf("Неизвестный рынок: %s", market)
	}

	reqURL := fmt.Sprintf("%s%s", baseURL, endpoint)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ошибка при запросе данных: %s, %s", resp.Status, string(bodyBytes))
	}

	var tickers []Ticker
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		return nil, err
	}

	var usdtTickers []Ticker
	for _, ticker := range tickers {
		if strings.HasSuffix(ticker.Symbol, "USDT") {
			usdtTickers = append(usdtTickers, ticker)
		}
	}

	switch timeFrame {
	case "5m", "15m", "1h", "4h":
		usdtTickers, err = calculatePriceChange(market, usdtTickers, timeFrame)
		if err != nil {
			return nil, err
		}
	case "24h":
	default:
		return nil, fmt.Errorf("Неподдерживаемый таймфрейм: %s", timeFrame)
	}

	return usdtTickers, nil
}

func calculatePriceChange(market string, tickers []Ticker, interval string) ([]Ticker, error) {
	var baseURL string
	var endpoint string

	if market == "spot" {
		baseURL = spotAPIEndpoint
		endpoint = "/api/v3/klines"
	} else if market == "futures" {
		baseURL = futuresAPIEndpoint
		endpoint = "/fapi/v1/klines"
	} else {
		return nil, fmt.Errorf("Неизвестный рынок: %s", market)
	}

	var updatedTickers []Ticker

	type result struct {
		ticker Ticker
		err    error
	}
	results := make(chan result, len(tickers))
	var wg sync.WaitGroup

	maxGoroutines := 10
	semaphore := make(chan struct{}, maxGoroutines)

	for _, ticker := range tickers {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(ticker Ticker) {
			defer wg.Done()
			defer func() { <-semaphore }()

			reqURL := fmt.Sprintf("%s%s?symbol=%s&interval=%s&limit=2", baseURL, endpoint, ticker.Symbol, interval)

			req, err := http.NewRequest("GET", reqURL, nil)
			if err != nil {
				results <- result{err: err}
				return
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("Ошибка при запросе данных для %s: %v\n", ticker.Symbol, err)
				results <- result{err: err}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Ошибка при запросе данных для %s: %s\n", ticker.Symbol, resp.Status)
				results <- result{err: fmt.Errorf("status code: %s", resp.Status)}
				return
			}

			var klines [][]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&klines); err != nil {
				fmt.Printf("Ошибка при парсинге данных для %s: %v\n", ticker.Symbol, err)
				results <- result{err: err}
				return
			}

			if len(klines) < 2 {
				results <- result{err: fmt.Errorf("недостаточно данных для %s", ticker.Symbol)}
				return
			}

			closePricePrev, _ := strconv.ParseFloat(klines[0][4].(string), 64)
			closePriceCurrent, _ := strconv.ParseFloat(klines[1][4].(string), 64)

			priceChangePercent := ((closePriceCurrent - closePricePrev) / closePricePrev) * 100

			ticker.PriceChangePercent = fmt.Sprintf("%.2f", priceChangePercent)
			results <- result{ticker: ticker}

		}(ticker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err == nil {
			updatedTickers = append(updatedTickers, res.ticker)
		}
	}

	return updatedTickers, nil
}

func calculateMaxDumpPump(market, symbol string) (maxDump float64, maxPump float64, err error) {
	var baseURL, endpoint string

	if market == "spot" {
		baseURL = spotAPIEndpoint
		endpoint = "/api/v3/klines"
	} else if market == "futures" {
		baseURL = futuresAPIEndpoint
		endpoint = "/fapi/v1/klines"
	} else {
		return 0, 0, fmt.Errorf("Неизвестный рынок: %s", market)
	}

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", "1h")
	params.Set("limit", "24")

	reqURL := fmt.Sprintf("%s%s?%s", baseURL, endpoint, params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("Ошибка при запросе данных свечей: %s, %s", resp.Status, string(bodyBytes))
		return
	}

	var klines [][]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&klines); err != nil {
		return
	}

	var maxPrice, minPrice float64

	for _, k := range klines {
		highPrice, _ := strconv.ParseFloat(k[2].(string), 64)
		lowPrice, _ := strconv.ParseFloat(k[3].(string), 64)

		if maxPrice == 0 || highPrice > maxPrice {
			maxPrice = highPrice
		}
		if minPrice == 0 || lowPrice < minPrice {
			minPrice = lowPrice
		}
	}

	currentPrice, _ := strconv.ParseFloat(klines[len(klines)-1][4].(string), 64)

	maxDump = ((currentPrice - minPrice) / currentPrice) * 100
	maxPump = ((maxPrice - currentPrice) / currentPrice) * 100

	return maxDump, maxPump, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
