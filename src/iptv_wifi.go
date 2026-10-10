package main

import (
 "crypto/rand"
 "encoding/base32"
 "errors"
 "fmt"
 "html"
 "net"
 "net/http"
 "strings"
 "sync"
 "time"
)

const iptvShareLifetime = 5*time.Minute

type iptvWifiShare struct{
 URL string
 Expires time.Time
 server *http.Server
 once sync.Once
}
func (s *iptvWifiShare) Close() {
 if s==nil{return}
 s.once.Do(func(){_ = s.server.Close()})
}
func iptvPrivateIPv4()(string,error){
 interfaces,err:=net.Interfaces()
 if err!=nil{return "",err}
 // Prefer Wi-Fi interface if available, falling back to any private
 // non-loopback IPv4 address. Never bind to a public Internet address.
 for pass:=0;pass<2;pass++ {
  for _,it:=range interfaces {
   if it.Flags&net.FlagUp==0||it.Flags&net.FlagLoopback!=0 {continue}
   n:=strings.ToLower(it.Name)
   if pass==0&&!strings.HasPrefix(n,"wlan")&&!strings.HasPrefix(n,"wlp"){continue}
   addrs,err:=it.Addrs()
   if err!=nil{continue}
   for _,addr:=range addrs {
    var ip net.IP
    switch x:=addr.(type) {
    case *net.IPNet:ip=x.IP
    case *net.IPAddr:ip=x.IP
    }
    ip4:=ip.To4()
    if ip4!=nil && ip4.IsPrivate() {return ip4.String(),nil}
   }
  }
 }
 return "",errors.New("không thấy Wi-Fi/LAN nội bộ; hãy kết nối Brick Pro và điện thoại cùng mạng Wi-Fi")
}
func iptvSharePage(report string)string{
 // Page has no third-party scripts, trackers, or external assets.
 return "<!doctype html><html lang=\"vi\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>BINANCE IPTV Diagnostic</title><style>body{font:16px system-ui,sans-serif;max-width:760px;margin:24px auto;padding:0 16px;background:#111827;color:#f3f4f6}textarea{width:100%;box-sizing:border-box;height:56vh;padding:12px;font:13px monospace;background:#f9fafb;color:#111}button{font:inherit;padding:13px;margin:12px 0;border:0;border-radius:8px;background:#eab308;color:#111827}p{line-height:1.5}</style></head><body><h2>BINANCE - Báo cáo IPTV</h2><p>Chia sẻ giữa Brick Pro và điện thoại qua Wi-Fi nội bộ. Không tự gửi ra Internet.</p><button id=\"copy\" type=\"button\">Sao chép báo cáo</button><p id=\"hint\">Bạn có thể chạm giữ để sao chép báo cáo và dán vào ChatGPT.</p><textarea id=\"report\" readonly>"+html.EscapeString(report)+"</textarea><script>document.getElementById(\"copy\").addEventListener(\"click\",function(){var t=document.getElementById(\"report\");t.focus();t.select();var ok=document.execCommand(\"copy\");document.getElementById(\"hint\").textContent=ok?\"Đã sao chép. Hãy dán báo cáo vào ChatGPT.\":\"Hãy chạm giữ để chọn và sao chép báo cáo.\";});</script></body></html>"
}
func iptvStartWifiShareOn(ip,report string,ttl time.Duration)(*iptvWifiShare,error){
 parsed:=net.ParseIP(ip)
 if parsed==nil||parsed.To4()==nil{return nil,errors.New("địa chỉ mạng không hợp lệ")}
 if !parsed.IsPrivate()&&!parsed.IsLoopback(){return nil,errors.New("không chia sẻ báo cáo trên IP công khai")}
 if ttl<=0||ttl>15*time.Minute{return nil,errors.New("thời gian chia sẻ không hợp lệ")}
 secret:=make([]byte,12)
 if _,err:=rand.Read(secret);err!=nil{return nil,err}
 token:=strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret))
 listener,err:=net.Listen("tcp",net.JoinHostPort(ip,"0"))
 if err!=nil{return nil,fmt.Errorf("không mở được Wi-Fi báo cáo: %w",err)}
 port:=listener.Addr().(*net.TCPAddr).Port
 path:="/r/"+token
 share:=&iptvWifiShare{URL:fmt.Sprintf("http://%s:%d%s",ip,port,path),Expires:time.Now().Add(ttl)}
 htmlPage:=iptvSharePage(report)
 mux:=http.NewServeMux()
 mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.Header().Set("Referrer-Policy","no-referrer")
  w.Header().Set("X-Frame-Options","DENY")
  if time.Now().After(share.Expires){http.Error(w,"Expired",http.StatusGone);return}
  if r.Method!="GET" || r.URL.Path!=path {http.NotFound(w,r);return}
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  _,_=w.Write([]byte(htmlPage))
 })
 share.server=&http.Server{
  Handler:mux,
  ReadHeaderTimeout:3*time.Second,
  ReadTimeout:5*time.Second,
  WriteTimeout:5*time.Second,
  IdleTimeout:5*time.Second,
  MaxHeaderBytes:4096,
 }
 go func(){_ = share.server.Serve(listener)}()
 go func(){timer:=time.NewTimer(ttl);defer timer.Stop();<-timer.C;share.Close()}()
 return share,nil
}
func iptvStartWifiShare(report string)(*iptvWifiShare,error){
 ip,err:=iptvPrivateIPv4()
 if err!=nil{return nil,err}
 return iptvStartWifiShareOn(ip,report,iptvShareLifetime)
}
