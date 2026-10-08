package main

import (
 "errors"
 "fmt"
 "html"
 "io"
 "net/http"
 "regexp"
 "strconv"
 "strings"
 "time"
)

const gold24HURL = "https://www.24h.com.vn/gia-vang-hom-nay-c425.html"
const goldRefreshInterval = 5*time.Minute

// Giá từ 24h.com.vn tính theo nghìn đồng/lượng.
type goldQuote struct {Name string; Buy,Sell float64}
type goldSnapshot struct {Quotes []goldQuote; SourceUpdate string; FetchedAt time.Time}
type goldResult struct {Snapshot goldSnapshot; Err error}

var (
 goldTableRowRe=regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr\s*>`)
 goldTableCellRe=regexp.MustCompile(`(?is)<t[dh]\b[^>]*>(.*?)</t[dh]\s*>`)
 goldHTMLTagsRe=regexp.MustCompile(`(?is)<[^>]+>`)
 goldScriptsRe=regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)\s*>`)
 goldBreakRe=regexp.MustCompile(`(?is)</(?:tr|td|th|div|p|h[1-6]|li|span)\s*>|<br\s*/?>`)
 goldSpaceRe=regexp.MustCompile(`\s+`)
 goldPriceRe=regexp.MustCompile(`\b\d{2,3}[.,]\d{3}\b`)
 goldUpdateRe=regexp.MustCompile(`(?i)Cập nhật lúc\s*([0-2]?\d:[0-5]\d\s*\(\d{2}/\d{2}/\d{4}\))`)
)
var goldBrands=[]struct{Name,Label string}{
 {"SJC","SJC"},{"DOJI HN","DOJI HN"},{"DOJI SG","DOJI SG"},
 {"BTMH","BTMH"},{"BTMC VRTL","BTMC VRTL"},{"BTMC SJC","BTMC SJC"},
 {"PHÚ QUÝ SJC","PHÚ QUÝ SJC"},{"PNJ TP.HCM","PNJ TP.HCM"},{"PNJ HÀ NỘI","PNJ HÀ NỘI"},
}
func goldText(raw string) string {
 t:=html.UnescapeString(goldHTMLTagsRe.ReplaceAllString(raw," "))
 return strings.TrimSpace(goldSpaceRe.ReplaceAllString(t," "))
}
func goldBrandLabel(s string)string{
 s=strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(s))," "))
 s=strings.ReplaceAll(s,"QÚY","QUÝ")
 s=strings.ReplaceAll(s,"QUY","QUÝ")
 for _,b:=range goldBrands {
  if s==strings.ReplaceAll(b.Name,"QÚY","QUÝ") {return b.Label}
 }
 return ""
}
func goldPriceValue(s string)(float64,bool){
 raw:=goldPriceRe.FindString(s)
 if raw=="" {return 0,false}
 raw=strings.NewReplacer(",","",".","").Replace(raw)
 value,err:=strconv.ParseFloat(raw,64)
 if err!=nil||value<50000||value>999999 {return 0,false}
 return value,true
}
// Do not present fabricated values if markup changes or an anti-bot page is returned.
func parseGold24H(page string)(goldSnapshot,error){
 if len(page)>5*1024*1024 {return goldSnapshot{},errors.New("trang giá vàng quá lớn")}
 clean:=goldScriptsRe.ReplaceAllString(page," ")
 snap:=goldSnapshot{}
 if m:=goldUpdateRe.FindStringSubmatch(goldText(clean));len(m)>1 {snap.SourceUpdate=m[1]}
 quotes:=make(map[string]goldQuote)
 for _,match:=range goldTableRowRe.FindAllStringSubmatch(clean,-1) {
  if len(match)<2 {continue}
  cells:=goldTableCellRe.FindAllStringSubmatch(match[1],-1)
  if len(cells)<3 {continue}
  label:=""
  for _,c:=range cells[:min(2,len(cells))] {
   if label=goldBrandLabel(goldText(c[1]));label!="" {break}
  }
  if label=="" {continue}
  values:=make([]float64,0,4)
  for _,cell:=range cells[1:] {
   if v,ok:=goldPriceValue(goldText(cell[1]));ok {values=append(values,v)}
  }
  if len(values)>=2 && values[0]<=values[1] {
   quotes[label]=goldQuote{Name:label,Buy:values[0],Sell:values[1]}
  }
 }
 if len(quotes)<2 {
  split:=goldBreakRe.ReplaceAllString(clean,"\n")
  text:=html.UnescapeString(goldHTMLTagsRe.ReplaceAllString(split," "))
  if i:=strings.Index(strings.ToUpper(text),"HÔM NAY");i>=0 {text=text[i:]}
  if i:=strings.Index(strings.ToUpper(text),"ĐƠN VỊ:");i>=0 {text=text[:i]}
  lines:=strings.Split(text,"\n")
  for i:=0;i<len(lines);i++ {
   brand:=goldBrandLabel(strings.TrimSpace(lines[i]))
   if brand=="" {continue}
   if _,ok:=quotes[brand];ok {continue}
   prices:=make([]float64,0,2)
   for j:=i+1;j<len(lines)&&j<=i+10&&len(prices)<2;j++ {
    next:=goldText(lines[j])
    if goldBrandLabel(next)!="" {break}
    if v,ok:=goldPriceValue(next);ok {prices=append(prices,v)}
   }
   if len(prices)==2&&prices[0]<=prices[1] {quotes[brand]=goldQuote{Name:brand,Buy:prices[0],Sell:prices[1]}}
  }
 }
 for _,b:=range goldBrands {
  if q,ok:=quotes[b.Label];ok {snap.Quotes=append(snap.Quotes,q)}
 }
 if len(snap.Quotes)==0 {return goldSnapshot{},errors.New("24h.com.vn không có bảng giá có thể đọc")}
 return snap,nil
}
func fetchGold24H(client *http.Client)(goldSnapshot,error){
 req,err:=http.NewRequest(http.MethodGet,gold24HURL,nil)
 if err!=nil {return goldSnapshot{},err}
 req.Header.Set("User-Agent","Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36")
 req.Header.Set("Accept","text/html,application/xhtml+xml")
 req.Header.Set("Accept-Language","vi-VN,vi;q=0.9")
 resp,err:=client.Do(req)
 if err!=nil {return goldSnapshot{},err}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK {return goldSnapshot{},fmt.Errorf("24h HTTP %d",resp.StatusCode)}
 if ct:=resp.Header.Get("Content-Type");ct!=""&&!strings.Contains(strings.ToLower(ct),"html") {
  return goldSnapshot{},errors.New("24h không trả về HTML")
 }
 body,err:=io.ReadAll(io.LimitReader(resp.Body,5*1024*1024+1))
 if err!=nil {return goldSnapshot{},err}
 snap,err:=parseGold24H(string(body))
 if err!=nil {return goldSnapshot{},err}
 snap.FetchedAt=time.Now()
 return snap,nil
}
func renderBusiness(fb *framebuffer,snap goldSnapshot,sel int,loading bool,errMsg string){
 fb.fill(cBg)
 margin:=max(18,fb.w/50)
 headerH:=max(112,fb.h*15/100)
 footerH:=bottomTabsHeight(fb)
 fb.rect(0,0,fb.w,headerH,cPanel)
 drawASCII(fb,margin,12,3,"KINH DOANH",cYellow)
 drawASCII(fb,margin,53,2,"GIÁ VÀNG - 24H.COM.VN",cText)
 status,sc:="DỮ LIỆU 24H",cGreen
 if loading {status,sc="ĐANG CẬP NHẬT...",cMuted
 }else if errMsg!="" {status,sc="LỖI NGUỒN 24H",cRed
 }else if snap.FetchedAt.IsZero() {status,sc="CHƯA CÓ DỮ LIỆU",cMuted
 }else if time.Since(snap.FetchedAt)>20*time.Minute {status,sc="DỮ LIỆU CŨ",cRed}
 drawASCII(fb,margin,79,1,status,sc)
 if len(snap.Quotes)==0 {
  msg:="CHƯA TẢI ĐƯỢC GIÁ VÀNG"
  drawASCII(fb,fb.w/2-asciiWidth(2,msg)/2,headerH+100,2,msg,cMuted)
  drawASCII(fb,margin,headerH+150,1,"X: THỬ LẠI KHI CÓ WI-FI",cYellow)
  drawBottomTabs(fb,pageBusiness,"L1/R1: ĐỔI TAB  X: TẢI LẠI")
  return
 }
 y:=headerH+12
 if snap.SourceUpdate!="" {drawASCII(fb,margin,y,1,"NGUỒN CẬP NHẬT: "+snap.SourceUpdate,cMuted)
 }else {drawASCII(fb,margin,y,1,"GIỜ TẢI: "+snap.FetchedAt.Format("15:04 02/01/2006"),cMuted)}
 if errMsg!="" {drawASCII(fb,margin,y+18,1,"ĐANG HIỂN THỊ GIÁ ĐÃ TẢI TRƯỚC ĐÓ",cRed)}
 y+=42
 priceW:=asciiWidth(2,"000.000")
 sellX:=fb.w-margin-priceW
 buyX:=sellX-priceW-34
 drawASCII(fb,margin,y,2,"THƯƠNG HIỆU",cMuted)
 drawASCII(fb,buyX,y,2,"MUA",cGreen)
 drawASCII(fb,sellX,y,2,"BÁN",cYellow)
 y+=38
 bottom:=fb.h-footerH-9
 rowH:=max(55,min(76,(bottom-y)/6))
 visible:=max(1,(bottom-y)/rowH)
 start:=0
 if sel>=visible {start=sel-visible+1}
 if start>max(0,len(snap.Quotes)-visible) {start=max(0,len(snap.Quotes)-visible)}
 end:=min(len(snap.Quotes),start+visible)
 for i:=start;i<end;i++ {
  q:=snap.Quotes[i]
  ry:=y+(i-start)*rowH
  bg:=cPanel
  if i==sel {bg=cPanel2}
  fb.rect(margin,ry,fb.w-2*margin,rowH-5,bg)
  if i==sel {fb.rect(margin,ry,4,rowH-5,cYellow)}
  scale:=2
  if asciiWidth(2,q.Name)>buyX-margin-10 {scale=1}
  drawASCII(fb,margin+12,ry+18,scale,q.Name,cText)
  drawASCII(fb,buyX,ry+18,2,fmt.Sprintf("%.3f",q.Buy/1000),cGreen)
  drawASCII(fb,sellX,ry+18,2,fmt.Sprintf("%.3f",q.Sell/1000),cYellow)
 }
 drawASCII(fb,margin,bottom-12,1,"ĐƠN VỊ: TRIỆU ĐỒNG/LƯỢNG",cMuted)
 drawBottomTabs(fb,pageBusiness,"L1/R1: ĐỔI TAB  LÊN/XUỐNG: CUỘN  X: TẢI LẠI")
}
