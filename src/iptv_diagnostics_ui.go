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
func drawIPTVQRCode(fb *framebuffer,modules [][]bool,x,y,maxSize int)int{
 if len(modules)==0 || maxSize<48{return 0}
 count:=len(modules)
 cell:=maxSize/count
 if cell<2{return 0}
 size:=count*cell
 // QR bitmap includes a quiet zone; preserve high-contrast black/white.
 fb.rect(x-3,y-3,size+6,size+6,color{255,255,255})
 for row,bits:=range modules {
  for col,dark:=range bits {
   if dark {fb.rect(x+col*cell,y+row*cell,cell,cell,color{0,0,0})}
  }
 }
 return size
}
func drawIPTVDiagnosticPage(fb *framebuffer,report string,scroll int,share *iptvWifiShare,message string){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageIPTV)
 margin:=max(18,fb.w/50)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,top+8,3,"IPTV - CHAN DOAN",cYellow)
 drawASCII(fb,margin,top+43,1,"KHONG CAN MAY TINH - DIEN THOAI CUNG WIFI",cMuted)
 y:=top+67
 if share!=nil&&time.Now().Before(share.Expires){
  // Show the short, memorable manual address and PIN on the left,
  // plus a self-contained QR (full secret URL) on the right.
  qrX:=fb.w-margin-235
  if qrX<fb.w/2 {qrX=fb.w/2}
  qrY:=top+78
  qrMax:=min(228,fb.w-margin-qrX,fb.h-footer-qrY-25)
  qrSize:=drawIPTVQRCode(fb,share.QR,qrX,qrY,qrMax)
  drawASCII(fb,margin,y,2,"QUET QR BANG DIEN THOAI",cGreen);y+=37
  drawASCII(fb,margin,y,1,"NEU KHONG QUET DUOC, MO LINK NGAN:",cText);y+=25
  linkScale:=2
  if asciiWidth(linkScale,share.ShortURL)>qrX-margin-15{linkScale=1}
  drawASCII(fb,margin,y,linkScale,share.ShortURL,cYellow);y+=39
  drawASCII(fb,margin,y,2,"MA PIN 6 SO:",cMuted);y+=30
  drawASCII(fb,margin,y,4,share.PIN,cYellow);y+=44
  drawASCII(fb,margin,y,1,"LINK TU DONG TAT SAU 5 PHUT",cMuted);y+=25
  if qrSize>0 {y=max(y,qrY+qrSize+16)}
 }else{
  drawASCII(fb,margin,y,1,"A: HIEN MA QR VA LINK WIFI NGAN (5 PHUT)",cText);y+=24
  drawASCII(fb,margin,y,1,"HOAC CHUP ANH BAO CAO BEN DUOI",cMuted);y+=24
 }
 if message!="" {
  drawASCII(fb,margin,y,1,cutNews(message,max(30,(fb.w-2*margin)/8)),cYellow)
  y+=23
 }
 fb.rect(margin,y,fb.w-2*margin,2,cPanel2)
 y+=12
 lines:=strings.Split(strings.TrimSuffix(report,"\n"),"\n")
 visible:=max(1,(fb.h-footer-y-5)/20)
 scroll=max(0,min(scroll,iptvDiagnosticScrollMax(report,visible)))
 for i:=scroll;i<len(lines)&&i<scroll+visible;i++{
  line:=strings.Map(func(r rune)rune{
   if r<32&&r!=9{return -1}
   return r
  },lines[i])
  drawASCII(fb,margin+5,y+(i-scroll)*20,1,cutNews(line,max(24,(fb.w-2*margin-20)/8)),cText)
 }
 drawBottomTabs(fb,pageIPTV,"A: QR / WIFI  X: CAP NHAT  LEN/XUONG: CUON  B: TRO LAI")
}
