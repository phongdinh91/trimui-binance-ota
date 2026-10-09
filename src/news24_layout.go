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
 drawASCIIRatio(fb,margin,10,9,2,title,cYellow)
 now:=time.Now()
 clock:=now.Format("15:04")
 date:=weekdayVI(now)+" "+now.Format("02/01/2006")
 drawASCII(fb,fb.w-asciiWidth(4,clock)-margin,8,4,clock,cText)
 drawASCII(fb,fb.w-asciiWidth(2,date)-margin,48,2,date,cMuted)
 return headerH
}

func footerParts(hint string) []string {
 // Each button label receives an independent equal-width slot.
 keys:=map[string]bool{"A:":true,"B:":true,"X:":true,"Y:":true,"START:":true,"SELECT:":true,"L1/R1:":true,"D-PAD:":true}
 words:=strings.Fields(hint)
 var parts []string
 for _,word:=range words{
  if keys[word]{parts=append(parts,word);continue}
  if len(parts)==0{parts=append(parts,word)}else{parts[len(parts)-1]+=" "+word}
 }
 return parts
}

func drawEvenFooterHints(fb *framebuffer,hint string,y int) {
 parts:=footerParts(hint)
 if len(parts)==0{return}
 margin:=max(12,fb.w/70)
 width:=fb.w-2*margin
 slot:=width/len(parts)
 for i,part:=range parts {
  scale:=2
  if asciiWidth(scale,part)>slot-6{scale=1}
  part=cutNews(part,max(8,(slot-8)/(7*scale)))
  x:=margin+i*slot+max(0,(slot-asciiWidth(scale,part))/2)
  drawASCII(fb,x,y,scale,part,cMuted)
 }
}
