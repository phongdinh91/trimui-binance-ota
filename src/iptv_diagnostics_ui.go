package main

import (
 "fmt"
 "strings"
 "time"
)

func iptvDiagnosticScrollMax(report string,visible int)int{
 n:=len(strings.Split(strings.TrimSuffix(report,"\n"),"\n"))
 if n<=visible{return 0}
 return n-visible
}
func drawIPTVDiagnosticPage(fb *framebuffer,report string,scroll int,share *iptvWifiShare,message string){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageIPTV)
 margin:=max(18,fb.w/50)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,top+8,3,"IPTV - CHAN DOAN",cYellow)
 drawASCII(fb,margin,top+43,1,"KIEM TRA MAY KHONG CAN MAY TINH",cMuted)
 y:=top+65
 if share!=nil && time.Now().Before(share.Expires){
  drawASCII(fb,margin,y,2,"CHIA SE WIFI DANG BAT",cGreen);y+=31
  drawASCII(fb,margin,y,1,"DIEN THOAI CUNG WIFI, MO TRINH DUYET:",cText);y+=19
  drawASCII(fb,margin,y,2,"http://",cYellow);y+=25
  address:=strings.TrimPrefix(share.URL,"http://")
  drawASCII(fb,margin,y,2,cutNews(address,max(28,(fb.w-2*margin)/14)),cYellow);y+=33
  secs:=int(time.Until(share.Expires).Seconds())
  drawASCII(fb,margin,y,1,fmt.Sprintf("LINK TU DONG DONG SAU %d GIAY",max(0,secs)),cMuted);y+=27
 }else{
  drawASCII(fb,margin,y,1,"A: BAT CHIA SE BAO CAO QUA WIFI (5 PHUT)",cText);y+=24
  drawASCII(fb,margin,y,1,"HOAC CHUP ANH MAN HINH BEN DUOI",cMuted);y+=24
 }
 if message!=""{drawASCII(fb,margin,y,1,cutNews(message,100),cYellow);y+=28}
 fb.rect(margin,y,fb.w-2*margin,2,cPanel2)
 y+=12
 lines:=strings.Split(strings.TrimSuffix(report,"\n"),"\n")
 available:=fb.h-footer-y-6
 visible:=max(1,available/20)
 if scroll<0{scroll=0}
 maxScroll:=iptvDiagnosticScrollMax(report,visible)
 if scroll>maxScroll{scroll=maxScroll}
 for i:=scroll;i<len(lines)&&i<scroll+visible;i++{
  row:=strings.Map(func(r rune)rune{
   if r<32 && r!=9{return -1}
   return r
  },lines[i])
  drawASCII(fb,margin+5,y+(i-scroll)*20,1,cutNews(row,max(24,(fb.w-2*margin-20)/8)),cText)
 }
 drawBottomTabs(fb,pageIPTV,"A: WIFI BAT/TAT  X: CAP NHAT  LEN/XUONG: CUON  B: TRO LAI")
}
