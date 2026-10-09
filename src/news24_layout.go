package main

import (
 "strings"
 "time"
)

// Identical layout, type sizes and colors across all three top-level tabs.
func drawAppTopBar(fb *framebuffer, page int) int {
 margin:=max(18,fb.w/50)
 headerH:=max(112,fb.h*15/100)
 fb.rect(0,0,fb.w,headerH,cPanel)
 title:="BINANCE"
 if page==pageBusiness{title="KINH DOANH"}
 if page==pageHitech{title="HI-TECH"}
 if page==pageSettings{title="CÀI ĐẶT"}
 drawASCIIRatio(fb,margin,10,9,2,title,cYellow)
 if page==pageBusiness||page==pageHitech {
  drawASCII(fb,margin,72,2,"24h.com.vn",cMuted)
 }
 now:=time.Now()
 clock:=now.Format("15:04")
 date:=weekdayVI(now)+" "+now.Format("02/01/2006")
 drawASCII(fb,fb.w-asciiWidth(4,clock)-margin,8,4,clock,cText)
 drawASCII(fb,fb.w-asciiWidth(2,date)-margin,48,2,date,cMuted)
 drawASCII(fb,fb.w-asciiWidth(1,appVersion)-margin,80,1,appVersion,cMuted)
 return headerH
}

func footerParts(hint string) []string {
 // Each button label receives an independent equal-width slot.
 keys:=map[string]bool{"A:":true,"B:":true,"X:":true,"Y:":true,"START:":true,"SELECT:":true,"L1/R1:":true,"D-PAD:":true,"MENU:":true,"LÊN/XUỐNG:":true,"TRÁI/PHẢI:":true}
 words:=strings.Fields(hint)
 var parts []string
 for _,word:=range words{
  if keys[word]{parts=append(parts,word);continue}
  if len(parts)==0{parts=append(parts,word)}else{parts[len(parts)-1]+=" "+word}
 }
 return parts
}

// Keep each legend left aligned, with consistent breathing room after every key group.
func drawEvenFooterHints(fb *framebuffer,hint string,y int) {
 parts:=footerParts(hint)
 if len(parts)==0{return}
 margin:=max(15,fb.w/50)
 maxWidth:=fb.w-2*margin
 scale,gap:=2,32
 needed:=func(scale,gap int)int{
  n:=0;for _,p:=range parts{n+=asciiWidth(scale,p)+gap};return n-gap
 }
 if needed(scale,gap)>maxWidth{scale,gap=1,26}
 if needed(scale,gap)>maxWidth{gap=14}
 x:=margin
 for i,part:=range parts{
  room:=maxWidth-(x-margin)
  if room<=10{break}
  if asciiWidth(scale,part)>room {part=cutNews(part,max(2,(room-4)/(7*scale)))}
  drawASCII(fb,x,y,scale,part,cMuted)
  x+=asciiWidth(scale,part)
  if i+1<len(parts){x+=gap}
 }
}
