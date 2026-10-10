package main

import (
 "strings"
 "sync"

 "binancegia/third_party/qrcode"
)

const (
 iptvSectionAll = iota
 iptvSectionFootball
)
const (
 iptvOfficialEPLID="official:fpt-play-premier-league"
 iptvOfficialEPLURL="https://fptplay.vn/lich-thi-dau/ngoai-hang-anh"
)

// Official metadata only: this is not a playable stream URL or a DRM bypass.
// The legal broadcaster in Viet Nam is FPT Play (Jan 2026 - 2030/31).
func iptvOfficialEPLEntry() iptvChannel{
 return iptvChannel{
  ID:iptvOfficialEPLID,
  Name:"NGOAI HANG ANH - FPT PLAY (CHINH THUC)",
  URLs:nil,
 }
}
func iptvIsFootballChannel(name string)bool{
 n:=strings.ToLower(strings.Join(strings.Fields(name)," "))
 if n==""{return false}
 // The sports grouping is a discovery filter, not a rights assertion.
 for _,term:=range []string{
  "bóng đá","bong da","football","soccer","thể thao","the thao",
  "sports","sport "," sport","on football","on sports",
  "vtv5","vtv6","futbol","premier league",
 }{
  if strings.Contains(n,term){return true}
 }
 return false
}
func iptvVisibleChannels(channels []iptvChannel,section int)[]iptvChannel{
 if section!=iptvSectionFootball{return channels}
 result:=make([]iptvChannel,0,len(channels)+1)
 result=append(result,iptvOfficialEPLEntry())
 for _,ch:=range channels {
  if iptvIsFootballChannel(ch.Name)&&len(ch.URLs)>0 {
   result=append(result,ch)
  }
 }
 return result
}
func iptvIsOfficialEPLEntry(ch iptvChannel)bool {
 return ch.ID==iptvOfficialEPLID && len(ch.URLs)==0
}
var (
 iptvOfficialQROnce sync.Once
 iptvOfficialQRData [][]bool
)
func iptvOfficialQR()[][]bool{
 iptvOfficialQROnce.Do(func(){
  code,err:=qrcode.New(iptvOfficialEPLURL,qrcode.Medium)
  if err==nil{iptvOfficialQRData=code.Bitmap()}
 })
 return iptvOfficialQRData
}
