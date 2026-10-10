package main

import (
 "io"
 "net/http"
 "net/url"
 "strings"
 "testing"
 "time"
)

func TestIPTVDiagnosticDoesNotIncludePrivateConfig(t *testing.T) {
 report:=iptvDiagnosticText(t.TempDir())
 if !strings.Contains(report,"IPTV DIAGNOSTIC") || !strings.Contains(report,"Platform:") {
  t.Fatalf("diagnostic missing basic information: %q",report)
 }
 for _,private:=range []string{"Wi-Fi password","Authorization:","market-cache.json","favorites.json","settings.json","iptv.m3u"} {
  if strings.Contains(report,private) {t.Fatalf("unexpected private detail %q",private)}
 }
}
func TestIPTVDiagnosticScrollBounds(t *testing.T){
 if got:=iptvDiagnosticScrollMax("a\nb\nc\n",2);got!=1{t.Fatalf("got scroll=%d",got)}
 if got:=iptvDiagnosticScrollMax("a\nb\n",4);got!=0{t.Fatalf("short report scroll=%d",got)}
}
func TestIPTVWiFiReportRequiresUnpredictableURL(t *testing.T){
 report:="IPTV TEST\nData: <script>alert(1)</script>\n"
 share,err:=iptvStartWifiShareOn("127.0.0.1",report,5*time.Second)
 if err!=nil{t.Fatal(err)}
 defer share.Close()
 c:=&http.Client{Timeout:2*time.Second}
 resp,err:=c.Get(strings.Split(share.URL,"/r/")[0]+"/r/wrong-token")
 if err!=nil{t.Fatal(err)}
 _=resp.Body.Close()
 if resp.StatusCode!=http.StatusNotFound{t.Fatalf("secret not enforced: %d",resp.StatusCode)}
 resp,err=c.Get(share.URL)
 if err!=nil{t.Fatal(err)}
 b,err:=io.ReadAll(io.LimitReader(resp.Body,32768))
 _=resp.Body.Close()
 if err!=nil{t.Fatal(err)}
 if resp.StatusCode!=http.StatusOK{t.Fatalf("status=%d",resp.StatusCode)}
 if !strings.Contains(string(b),"IPTV TEST")||!strings.Contains(string(b),"&lt;script&gt;") {
  t.Fatalf("report escaped incorrectly: %q",string(b))
 }
 if strings.Contains(string(b),"Data: <script>"){t.Fatal("report HTML injection")}
 if resp.Header.Get("Cache-Control")!="no-store"{t.Fatal("diagnostic must not be cached")}
 if resp.Header.Get("X-Frame-Options")!="DENY"{t.Fatal("diagnostic must not be frameable")}
 if _,err:=iptvStartWifiShareOn("8.8.8.8","report",time.Minute);err==nil{
  t.Fatal("public IP must be rejected")
 }
 if _,err:=iptvStartWifiShareOn("127.0.0.1","report",30*time.Minute);err==nil{
  t.Fatal("unbounded exposure must be rejected")
 }
}


func TestIPTVShortLinkAndPINGate(t *testing.T){
 report:="LOCAL REPORT\nSecret status: video unsupported\n"
 share,err:=iptvStartWifiShareOn("127.0.0.1",report,5*time.Second)
 if err!=nil{t.Fatal(err)}
 defer share.Close()
 if !strings.HasPrefix(share.ShortURL,"http://127.0.0.1:"){t.Fatal("short URL missing")}
 if strings.Contains(share.ShortURL,"/r/"){t.Fatal("short URL should have no random token")}
 if len(share.PIN)!=6{t.Fatalf("invalid PIN: %q",share.PIN)}
 for _,x:=range share.PIN{if x<'0'||x>'9'{t.Fatal("PIN must be numeric")}}
 if len(share.QR)<21 || len(share.QR[0])!=len(share.QR){t.Fatal("QR matrix malformed")}
 dark:=0
 for _,row:=range share.QR{for _,bit:=range row{if bit{dark++}}}
 if dark<90{t.Fatalf("expected QR modules, got %d",dark)}
 client:=&http.Client{Timeout:2*time.Second}
 response,err:=client.Get(share.ShortURL)
 if err!=nil{t.Fatal(err)}
 landing,_:=io.ReadAll(response.Body)
 _=response.Body.Close()
 if response.StatusCode!=200||!strings.Contains(string(landing),"Mã PIN") {
  t.Fatalf("no PIN landing form: %d",response.StatusCode)
 }
 if strings.Contains(string(landing),"Secret status:"){t.Fatal("report leaked on short link")}
 wrong,err:=client.PostForm(share.ShortURL,url.Values{"pin":{"000000"}})
 if err!=nil{t.Fatal(err)}
 _=wrong.Body.Close()
 if wrong.StatusCode!=http.StatusForbidden && share.PIN!="000000"{
  t.Fatalf("invalid PIN status %d",wrong.StatusCode)
 }
 correct,err:=client.PostForm(share.ShortURL,url.Values{"pin":{share.PIN}})
 if err!=nil{t.Fatal(err)}
 b,err:=io.ReadAll(correct.Body)
 _=correct.Body.Close()
 if err!=nil{t.Fatal(err)}
 if correct.StatusCode!=200||!strings.Contains(string(b),"Secret status:") {
  t.Fatalf("correct PIN did not reveal report: %d",correct.StatusCode)
 }
}
func TestIPTVPINRateLimitAndExpiration(t *testing.T){
 share,err:=iptvStartWifiShareOn("127.0.0.1","NEVER PUBLIC\n",250*time.Millisecond)
 if err!=nil{t.Fatal(err)}
 defer share.Close()
 client:=&http.Client{Timeout:time.Second}
 wrong:="abcdef"
 for i:=0;i<8;i++{
  resp,err:=client.PostForm(share.ShortURL,url.Values{"pin":{wrong}})
  if err!=nil{t.Fatal(err)}
  _=resp.Body.Close()
  if resp.StatusCode!=http.StatusForbidden{t.Fatalf("wrong PIN request %d returned %d",i,resp.StatusCode)}
 }
 response,err:=client.PostForm(share.ShortURL,url.Values{"pin":{share.PIN}})
 if err!=nil{t.Fatal(err)}
 _=response.Body.Close()
 if response.StatusCode!=http.StatusTooManyRequests{t.Fatalf("limit must reject even correct PIN after 8 failures: %d",response.StatusCode)}
 time.Sleep(400*time.Millisecond)
 expired,err:=client.Get(share.URL)
 if err==nil{
  _=expired.Body.Close()
  if expired.StatusCode==http.StatusOK {t.Fatal("expired report must not be served")}
 }
}
