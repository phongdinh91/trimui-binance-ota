package main

import (
 "io"
 "net/http"
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
