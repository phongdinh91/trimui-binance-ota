package main

import (
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
)
func TestGoldTableHTML(t *testing.T) {
 page:=`<h1>Giá vàng</h1><p>Cập nhật lúc 19:14 (19/06/2026)</p><table>
 <tr><td>SJC</td><td>143,700<img alt="tăng"></td><td>146,700</td><td>148,800</td><td>151,300</td></tr>
 <tr><td>DOJI HN</td><td>142,500</td><td>146,500</td></tr>
 <tr><td>PNJ TP.HCM</td><td>143,600</td><td>146,600</td></tr></table>`
 got,err:=parseGold24H(page)
 if err!=nil {t.Fatal(err)}
 if len(got.Quotes)!=3 {t.Fatalf("expected 3 quotes, got %+v",got.Quotes)}
 if got.Quotes[0].Name!="SJC"||got.Quotes[0].Buy!=143700||got.Quotes[0].Sell!=146700 {t.Fatalf("incorrect SJC: %+v",got.Quotes[0])}
 if got.SourceUpdate!="19:14 (19/06/2026)" {t.Fatalf("date=%q",got.SourceUpdate)}
}
func TestGoldDivHTML(t *testing.T) {
 page:=`<div>Hôm nay</div><div>SJC</div><span>143,700</span><span>146,700</span>
 <div>BTMC VRTL</div><span>142,000</span><span>147,000</span><div>Đơn vị: nghìn đồng/lượng</div>`
 got,err:=parseGold24H(page)
 if err!=nil {t.Fatal(err)}
 if len(got.Quotes)!=2||got.Quotes[1].Name!="BTMC VRTL" {t.Fatalf("bad fallback %+v",got.Quotes)}
}
func TestGoldRejectsInvalid(t *testing.T) {
 for _,s:=range []string{"",`<html>Security check</html>`,`<td>SJC</td><td>0.1</td>`} {
  if _,err:=parseGold24H(s);err==nil {t.Fatalf("accepted bad source %q",s)}
 }
 if v,ok:=goldPriceValue("143,700");!ok||v!=143700 {t.Fatalf("price=%v ok=%v",v,ok)}
 if _,ok:=goldPriceValue("0.000");ok {t.Fatal("accepted out-of-range price")}
}
type goldRoundTripFunc func(*http.Request)(*http.Response,error)
func (f goldRoundTripFunc) RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func TestGoldFetcherUses24h(t *testing.T) {
 client:=&http.Client{Timeout:time.Second,Transport:goldRoundTripFunc(func(r *http.Request)(*http.Response,error){
  if r.URL.String()!=gold24HURL {t.Fatalf("unexpected source %s",r.URL)}
  w:=httptest.NewRecorder()
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  w.WriteString(`<table><tr><td>SJC</td><td>143,700</td><td>146,700</td></tr></table>`)
  return w.Result(),nil
 })}
 got,err:=fetchGold24H(client)
 if err!=nil {t.Fatal(err)}
 if len(got.Quotes)!=1||!strings.EqualFold(got.Quotes[0].Name,"SJC")||got.FetchedAt.IsZero() {t.Fatalf("unexpected snapshot %+v",got)}
}
func TestThreeTabOrder(t *testing.T) {
 if cycleMainPage(pageSearch,1)!=pageFavorites||cycleMainPage(pageFavorites,1)!=pageBusiness||cycleMainPage(pageBusiness,1)!=pageSearch {t.Fatal("R1 tabs")}
 if cycleMainPage(pageSearch,-1)!=pageBusiness||cycleMainPage(pageBusiness,-1)!=pageFavorites {t.Fatal("L1 tabs")}
}
