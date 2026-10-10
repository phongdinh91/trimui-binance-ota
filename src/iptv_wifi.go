package main

import (
 "crypto/rand"
 "crypto/subtle"
 "encoding/base32"
 "math/big"
 "sync/atomic"

 "binancegia/third_party/qrcode"
 "errors"
 "fmt"
 "net"
 "net/http"
 "strings"
 "sync"
 "time"
)

const iptvShareLifetime = 5*time.Minute

type iptvWifiShare struct{
 URL string
 ShortURL string
 PIN string
 QR [][]bool
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
 return iptvMobileReportPage(report)
}
// A separate short, PIN-gated entry point avoids typing the long random
// URL, while QR scanning retains an unguessable direct link.
func iptvPINPage()string{
 return `<!doctype html><html lang="vi"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>BINANCE IPTV</title><style>body{font:18px system-ui;max-width:460px;margin:32px auto;padding:16px;background:#111827;color:white}input,button{font:22px system-ui;padding:12px;max-width:100%;border-radius:8px;box-sizing:border-box}input{width:100%}button{background:#eab308;color:#111827;border:0;margin-top:12px}p{line-height:1.5}</style></head><body><h2>Chẩn đoán BINANCE IPTV</h2><p>Nhập mã PIN 6 số đang hiển thị trên Brick Pro. Chỉ sử dụng được khi thiết bị đang bật chia sẻ Wi-Fi.</p><form method="POST" action="/"><label for="pin">Mã PIN</label><input type="text" name="pin" id="pin" inputmode="numeric" pattern="[0-9]{6}" minlength="6" maxlength="6" autocomplete="off" required><button type="submit">Xem báo cáo</button></form></body></html>`
}
func iptvStartWifiShareOn(ip,report string,ttl time.Duration)(*iptvWifiShare,error){
 parsed:=net.ParseIP(ip)
 if parsed==nil||parsed.To4()==nil{return nil,errors.New("địa chỉ mạng không hợp lệ")}
 if !parsed.IsPrivate()&&!parsed.IsLoopback(){return nil,errors.New("không chia sẻ báo cáo trên IP công khai")}
 if ttl<=0||ttl>15*time.Minute{return nil,errors.New("thời gian chia sẻ không hợp lệ")}
 secret:=make([]byte,12)
 if _,err:=rand.Read(secret);err!=nil{return nil,err}
 token:=strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret))
 pinNumber,err:=rand.Int(rand.Reader,big.NewInt(1000000))
 if err!=nil{return nil,err}
 pin:=fmt.Sprintf("%06d",pinNumber.Int64())
 // Prefer a stable port for easy manual typing; use an ephemeral port if busy.
 listener,err:=net.Listen("tcp",net.JoinHostPort(ip,"8765"))
 if err!=nil{listener,err=net.Listen("tcp",net.JoinHostPort(ip,"0"))}
 if err!=nil{return nil,fmt.Errorf("không mở được Wi-Fi báo cáo: %w",err)}
 port:=listener.Addr().(*net.TCPAddr).Port
 path:="/r/"+token
 shortURL:=fmt.Sprintf("http://%s:%d",ip,port)
 share:=&iptvWifiShare{
  URL:shortURL+path,ShortURL:shortURL,PIN:pin,Expires:time.Now().Add(ttl),
 }
 // Short URL QR leads to the tested PIN form, not the fragile long token.
 // A smaller QR matrix gives much larger square modules on Brick Pro.
 qr,err:=qrcode.New(share.ShortURL,qrcode.Low)
 if err!=nil{_ = listener.Close();return nil,fmt.Errorf("không tạo được mã QR: %w",err)}
 share.QR=qr.Bitmap()
 reportPage:=iptvSharePage(report)
 pinPage:=iptvPINPage()
 var failures atomic.Int32
 mux:=http.NewServeMux()
 mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.Header().Set("Referrer-Policy","no-referrer")
  w.Header().Set("X-Frame-Options","DENY")
  if time.Now().After(share.Expires){http.Error(w,"Expired",http.StatusGone);return}
  // QR's long random URL opens the report without any typing.
  if r.Method==http.MethodGet&&r.URL.Path==path {
   w.Header().Set("Content-Type","text/html; charset=utf-8")
   _,_=w.Write([]byte(reportPage))
   return
  }
  if r.URL.Path!="/" {http.NotFound(w,r);return}
  if r.Method==http.MethodGet{
   w.Header().Set("Content-Type","text/html; charset=utf-8")
   _,_=w.Write([]byte(pinPage))
   return
  }
  if r.Method!=http.MethodPost {http.Error(w,"Method not allowed",http.StatusMethodNotAllowed);return}
  if failures.Load()>=8{http.Error(w,"Too many attempts: restart sharing on Brick Pro",http.StatusTooManyRequests);return}
  r.Body=http.MaxBytesReader(w,r.Body,1024)
  if err:=r.ParseForm();err!=nil{http.Error(w,"Invalid form",http.StatusBadRequest);return}
  entered:=r.PostForm.Get("pin")
  if subtle.ConstantTimeCompare([]byte(entered),[]byte(pin))!=1 {
   failures.Add(1)
   http.Error(w,"Incorrect PIN",http.StatusForbidden)
   return
  }
  w.Header().Set("Content-Type","text/html; charset=utf-8")
  _,_=w.Write([]byte(reportPage))
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
