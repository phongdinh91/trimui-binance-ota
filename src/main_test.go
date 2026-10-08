package main

import "testing"

func TestChartIntervals(t *testing.T) {
	want := []string{"15m", "30m", "1h", "4h", "1d", "1w", "1M"}
	if len(chartRanges) != len(want) {
		t.Fatalf("interval count: got %d, want %d", len(chartRanges), len(want))
	}
	for i, w := range want {
		if chartRanges[i].Interval != w {
			t.Fatalf("chart %d: got %s, want %s", i, chartRanges[i].Interval, w)
		}
	}
}

func TestSearchPairPrefixes(t *testing.T) {
	xs := []ticker{
		{Symbol:"BNBBTC", BaseAsset:"BNB", QuoteAsset:"BTC"},
		{Symbol:"BNBUSDT", BaseAsset:"BNB", QuoteAsset:"USDT"},
		{Symbol:"ETHBTC", BaseAsset:"ETH", QuoteAsset:"BTC"},
	}
	match := searchPairs(xs, "BNBB", 10)
	if len(match) == 0 || match[0].Symbol != "BNBBTC" {
		t.Fatalf("BNBB should suggest BNB/BTC: %+v", match)
	}
	match = searchPairs(xs, "BNB/BTC", 10)
	if len(match) == 0 || match[0].Symbol != "BNBBTC" {
		t.Fatalf("slash form should match BNB/BTC: %+v", match)
	}
}

func TestIndicatorsHaveAlignedOutput(t *testing.T) {
	points := []float64{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30}
	if len(smaSeries(points, 5)) != len(points) { t.Fatal("SMA length") }
	dif, dea, hist := macdSeries(points)
	if len(dif) != len(points) || len(dea) != len(points) || len(hist) != len(points) {
		t.Fatal("MACD length")
	}
}
