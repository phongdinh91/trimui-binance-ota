package main

import "fmt"

func drawIPTVPage(fb *framebuffer,channels []iptvChannel,selected int,loading,playing bool,status string,playerReady bool){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageIPTV)
 margin:=max(18,fb.w/50)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,top+8,3,"IPTV - TRUYỀN HÌNH",cYellow)
 note:="DANH SÁCH KÊNH VIỆT NAM / IPTV-ORG"
 if loading{note="ĐANG CẬP NHẬT DANH SÁCH KÊNH..."}
 drawASCII(fb,margin,top+47,1,note,cMuted)
 if !playerReady{
  drawASCII(fb,margin,top+69,1,"CẦN CÀI MPV ĐỂ PHÁT HÌNH TRÊN BRICK PRO",cRed)
 }else{
  drawASCII(fb,margin,top+69,1,"MPV: SẴN SÀNG (CHƯA XÁC NHẬN VIDEO TRÊN MÁY)",cGreen)
 }
 contentTop:=top+94
 bottom:=fb.h-footer-6
 if len(channels)==0{
  drawASCII(fb,margin,contentTop+34,2,"CHƯA CÓ KÊNH IPTV",cMuted)
  if loading {drawASCII(fb,margin,contentTop+73,1,"ĐANG TẢI PLAYLIST...",cYellow)}
  if status!=""{drawASCII(fb,margin,contentTop+111,1,cutNews(status,96),cRed)}
  drawBottomTabs(fb,pageIPTV,"Y: CHẨN ĐOÁN   X: TẢI KÊNH   L1/R1: ĐỔI TAB")
  return
 }
 usable:=bottom-contentTop-46
 rowH:=max(44,min(70,usable/7))
 visible:=max(1,usable/rowH)
 if selected>=len(channels){selected=len(channels)-1}
 if selected<0{selected=0}
 start:=selected-visible+1
 if start<0{start=0}
 if start>len(channels)-visible{start=max(0,len(channels)-visible)}
 for i:=start;i<len(channels) && i<start+visible;i++{
  ch:=channels[i]
  y:=contentTop+(i-start)*rowH
  bg:=cPanel
  if i==selected{bg=cPanel2}
  fb.rect(margin,y,fb.w-2*margin,rowH-4,bg)
  if i==selected{fb.rect(margin,y,6,rowH-4,cYellow)}
  line:=fmt.Sprintf("%03d  %s",i+1,cutNews(ch.Name,max(32,(fb.w-190)/14)))
  drawASCII(fb,margin+17,y+10,2,line,cText)
  detail:=fmt.Sprintf("%d NGUỒN",len(ch.URLs))
  drawASCII(fb,fb.w-margin-asciiWidth(1,detail)-14,y+12,1,detail,cMuted)
 }
 statusColor:=cYellow
 if playing{statusColor=cGreen}
 if status!=""{
  drawASCII(fb,margin,bottom-37,1,cutNews(status,100),statusColor)
 }
 drawBottomTabs(fb,pageIPTV,"A: XEM   B: DỪNG   X: NGUỒN / TẢI LẠI   Y: CHẨN ĐOÁN")
}
