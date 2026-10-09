package main

import "testing"

func TestStartAlwaysReturnsToBinanceGrid(t *testing.T) {
 next,handled:=binanceStartTarget(pageSearch)
 if !handled||next!=pageFavorites {t.Fatalf("START in search must return BINANCE grid, got %d, %t",next,handled)}
 next,handled=binanceStartTarget(pageFavorites)
 if !handled||next!=pageSearch {t.Fatalf("START in BINANCE must open search, got %d, %t",next,handled)}
 for _,p:=range []int{pageBusiness,pageHitech} {
  next,handled=binanceStartTarget(p)
  if handled||next!=p{t.Fatalf("START must not affect 24h category page %d",p)}
 }
}
