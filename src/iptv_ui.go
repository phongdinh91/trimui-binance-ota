package main

import "fmt"

func drawIPTVPage(fb *framebuffer,channels []iptvChannel,selected,section int,loading,playing bool,status string,playerReady bool){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageIPTV)
 margin:=max(18,fb.w/50)
 footer:=bottomTabsHeight(fb)
 title:="IPTV - TRUYỀN HÌNH"
 note:="DANH SÁCH KÊNH VIỆT NAM / IPTV-ORG"
 if section==iptvSectionFootball{
  title="IPTV - BÓNG ĐÁ"
  note="FPT PLAY NHA / CÁC KÊNH THỂ THAO HIỆN CÓ"
 }
 drawASCII(fb,margin,top+8,3,title,cYellow)
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
  drawBottomTabs(fb,pageIPTV,"SELECT: BÓNG ĐÁ / TẤT CẢ   X: TẢI KÊNH   Y: CHẨN ĐOÁN")
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
  if iptvIsOfficialEPLEntry(ch){detail="QUÉT QR"}
  drawASCII(fb,fb.w-margin-asciiWidth(1,detail)-14,y+12,1,detail,cMuted)
 }
 statusColor:=cYellow
 if playing{statusColor=cGreen}
 if status!=""{
  drawASCII(fb,margin,bottom-37,1,cutNews(status,100),statusColor)
 }
 drawBottomTabs(fb,pageIPTV,"SELECT: BÓNG ĐÁ/TẤT CẢ   A: XEM/QR   X: TẢI LẠI   Y: CHẨN ĐOÁN")
}


// Informational guide: an authenticated FPT Play broadcast is not an open MPV URL.
// Scan with a phone that is entitled to use FPT Play. Never claim a free IPTV feed.
func drawIPTVFootballGuidePage(fb *framebuffer){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageIPTV)
 margin:=max(18,fb.w/50)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,top+12,3,"NGOẠI HẠNG ANH - FPT PLAY",cYellow)
 drawASCII(fb,margin,top+58,2,"PHÁT SÓNG CHÍNH THỨC TẠI VIỆT NAM",cText)
 drawASCII(fb,margin,top+89,1,"FPT PLAY: BẢN QUYỀN TỪ 2026 ĐẾN 2030/31",cMuted)
 drawASCII(fb,margin,top+118,1,"QUÉT MÃ QR BẰNG ĐIỆN THOẠI ĐỂ XEM LỊCH VÀ ĐĂNG NHẬP",cText)
 availableH:=fb.h-footer-top-188
 qrMax:=min(270,availableH-30,(fb.w-2*margin)/2)
 qrX:=fb.w-margin-qrMax-12
 qrY:=top+158
 size:=drawIPTVQRCode(fb,iptvOfficialQR(),qrX,qrY,qrMax)
 leftW:=qrX-margin-25
 textY:=top+160
 drawASCII(fb,margin,textY,2,"fptplay.vn",cYellow);textY+=42
 drawASCII(fb,margin,textY,1,"CẦN TÀI KHOẢN / GÓI BẢN QUYỀN",cMuted);textY+=30
 drawASCII(fb,margin,textY,1,"BINANCE KHÔNG CÓ LINK MPV CÔNG KHAI",cMuted);textY+=29
 drawASCII(fb,margin,textY,1,"CHO CÁC TRẬN NGOẠI HẠNG ANH",cMuted);textY+=33
 drawASCII(fb,margin,textY,1,"MÃ QR DẪN ĐẾN TRANG CHÍNH THỨC",cText)
 _=leftW
 _=size
 drawBottomTabs(fb,pageIPTV,"B: VỀ KÊNH BÓNG ĐÁ   SELECT: ĐỔI NHÓM   Y: CHẨN ĐOÁN")
}
