package main

// binanceStartTarget maps START to the opposite BINANCE view.
// It never opens an old detail screen; callers must clear detail on either transition.
func binanceStartTarget(page int) (next int, handled bool) {
 switch page {
 case pageSearch: return pageFavorites,true
 case pageFavorites: return pageSearch,true
 default: return page,false
 }
}
