package main

import (
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
 "strconv"
 "strings"
)

const (
 ledSystem=iota
 ledOff
 ledGold
 ledCyan
 ledPurple
 ledBreathing
 ledModeCount
)
var ledModeNames=[]string{"HỆ THỐNG","TẮT","VÀNG TĨNH","XANH TĨNH","TÍM TĨNH","NHỊP THỞ"}
var ledModeColors=[]string{"","000000","FFB900","00CCFF","B46AFF","FFB900"}

func applyTheme(light bool){
 if light {
  cBg=color{245,247,250}
  cPanel=color{226,232,239}
  cPanel2=color{206,218,234}
  cText=color{26,38,56}
  cMuted=color{76,92,115}
  cYellow=color{139,99,0}
  cGreen=color{0,130,75}
  cRed=color{191,43,56}
  cBlue=color{36,89,174}
  return
 }
 cBg=color{15,18,23}
 cPanel=color{25,29,36}
 cPanel2=color{34,39,48}
 cText=color{239,242,246}
 cMuted=color{156,163,175}
 cYellow=color{240,185,11}
 cGreen=color{22,199,132}
 cRed=color{234,57,67}
 cBlue=color{61,130,246}
}
func settingsNextLED(current,direction int)int{
 return (current+direction+ledModeCount)%ledModeCount
}
func ledDriverPath(name string)string{return filepath.Join("/sys/class/led_anim",name)}
func writeLEDNode(name,value string)error{
 if !regexp.MustCompile("^[a-z0-9_]+$").MatchString(name){return errors.New("LED không hợp lệ")}
 p:=ledDriverPath(name)
 if _,err:=os.Stat(p);err!=nil{return fmt.Errorf("Stock OS không hỗ trợ %s: %w",name,err)}
 return os.WriteFile(p,[]byte(value+"\n"),0644)
}
func writeLEDOptional(name,value string)error{
 if _,err:=os.Stat(ledDriverPath(name));os.IsNotExist(err){return nil}
 return writeLEDNode(name,value)
}
var ledIDRe=regexp.MustCompile("\\b([0-9]{1,2})\\b")
func ledBreathEffectID(driverHelp string)(string,bool){
 for _,line:=range strings.Split(driverHelp,"\n"){
  lower:=strings.ToLower(line)
  if !strings.Contains(lower,"breath")&&!strings.Contains(lower,"pulse"){continue}
  if m:=ledIDRe.FindStringSubmatch(line);len(m)==2{
   id,err:=strconv.Atoi(m[1])
   if err==nil&&id>=0&&id<=16{return m[1],true}
  }
 }
 return "",false
}
// Use documented sysfs nodes only. Default never touches OS LED settings.
func applyBrickLED(mode int)error{
 if mode==ledSystem{return nil}
 if mode<0||mode>=ledModeCount{return errors.New("chế độ LED không hợp lệ")}
 if _,err:=os.Stat(ledDriverPath("effect_enable"));err!=nil{
  return errors.New("không tìm thấy driver LED của Brick Pro (Stock OS)")
 }
 if mode==ledOff{
  if err:=writeLEDNode("max_scale","0");err!=nil{return err}
  for _,node:=range []string{"max_scale_lr","max_scale_f1f2","max_scale_rear"}{
   if err:=writeLEDOptional(node,"0");err!=nil{return err}
  }
  return nil
 }
 if mode==ledBreathing{
  effects,err:=os.ReadFile(ledDriverPath("effect_names"))
  if err!=nil{return errors.New("firmware không công bố danh sách hiệu ứng LED")}
  id,ok:=ledBreathEffectID(string(effects))
  if !ok{return errors.New("firmware không có hiệu ứng NHỊP THỞ tương thích")}
  if err:=writeLEDNode("max_scale","50");err!=nil{return err}
 for _,node:=range []string{"max_scale_lr","max_scale_f1f2","max_scale_rear"}{
  if err:=writeLEDOptional(node,"50");err!=nil{return err}
 }
  if err:=writeLEDNode("effect_rgb_hex_m",ledModeColors[mode]+" ");err!=nil{return err}
  if err:=writeLEDNode("effect_m",id);err!=nil{return err}
  if err:=writeLEDOptional("effect_cycles_m","-1");err!=nil{return err}
  return writeLEDNode("effect_enable","1")
 }
 if err:=writeLEDNode("max_scale","50");err!=nil{return err}
 for _,node:=range []string{"max_scale_lr","max_scale_f1f2","max_scale_rear"}{
  if err:=writeLEDOptional(node,"50");err!=nil{return err}
 }
 if err:=writeLEDNode("effect_rgb_hex_m",ledModeColors[mode]+" ");err!=nil{return err}
 if err:=writeLEDNode("effect_m","4");err!=nil{return err}
 if err:=writeLEDOptional("effect_cycles_m","-1");err!=nil{return err}
 for _,zone:=range []string{"f1","f2","lr","rear"}{
  if err:=writeLEDOptional("effect_rgb_hex_"+zone,ledModeColors[mode]+" ");err!=nil{return err}
  if err:=writeLEDOptional("effect_"+zone,"4");err!=nil{return err}
  if err:=writeLEDOptional("effect_cycles_"+zone,"-1");err!=nil{return err}
 }
 return writeLEDNode("effect_enable","1")
}

const (
 settingsOTA=0
 settingsTheme=1
 settingsLED=2
 settingsAbout=3
 settingsItemCount=4
)
func ledStatusMessage(mode int)string{
 if mode==ledSystem{return "HỆ THỐNG: KHỞI ĐỘNG LẠI ĐỂ KHÔI PHỤC"}
 return "ĐÃ CHỌN "+ledModeNames[mode]
}
func settingsValue(s settingsFile,idx int)string{
 switch idx {
 case settingsOTA:return "BẤM A ĐỂ KIỂM TRA"
 case settingsTheme:if s.ThemeLight{return "SÁNG"};return "TỐI"
 case settingsLED:return ledModeNames[s.LEDMode]
 case settingsAbout:return "ỨNG DỤNG & NHÀ PHÁT TRIỂN"
 }
 return ""
}
func drawSlideToggle(fb *framebuffer,x,y int,on bool){
 w,h:=94,42
 track:=cMuted
 if on{track=cGreen}
 fb.rect(x,y,w,h,track)
 knob:=cPanel2
 if on{knob=cText}
 xx:=x+7
 if on{xx=x+w-36}
 fb.rect(xx,y+6,29,h-12,knob)
}
func drawAppSettings(fb *framebuffer,s settingsFile,selection int,checkingOTA bool,ledStatus string){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageSettings)
 margin:=max(18,fb.w/50)
 drawASCII(fb,margin,top+8,2,"TÙY CHỈNH TRÊN BRICK PRO",cMuted)
 labels:=[]string{"CẬP NHẬT OTA","CHỦ ĐỀ GIAO DIỆN","HIỆU ỨNG LED","GIỚI THIỆU"}
 gap:=12
 yTop:=top+52
 footer:=bottomTabsHeight(fb)
 rowH:=max(70,min(112,(fb.h-footer-yTop-18)/settingsItemCount-gap))
 for i,label:=range labels{
  y:=yTop+i*(rowH+gap)
  bg:=cPanel;if selection==i{bg=cPanel2}
  fb.rect(margin,y,fb.w-margin*2,rowH,bg)
  if selection==i{fb.rect(margin,y,6,rowH,cYellow)}
  drawASCII(fb,margin+20,y+14,2,label,cText)
  caption:=settingsValue(s,i)
  if i==settingsOTA&&checkingOTA{caption="ĐANG KIỂM TRA GITHUB..."}
  if i==settingsLED&&ledStatus!=""{caption=cutNews(settingsValue(s,i)+" - "+ledStatus,62)}
  drawASCII(fb,margin+20,y+49,1,caption,cMuted)
  if i==settingsTheme{
   drawSlideToggle(fb,fb.w-margin-120,y+19,s.ThemeLight)
  }else{
   drawASCII(fb,fb.w-margin-30,y+25,2,">",cYellow)
  }
 }
 drawBottomTabs(fb,pageSettings,"A: CHỌN   LÊN/XUỐNG: DI CHUYỂN   TRÁI/PHẢI: ĐỔI LED")
}
func drawAboutApp(fb *framebuffer){
 fb.fill(cBg)
 top:=drawAppTopBar(fb,pageSettings)
 margin:=max(18,fb.w/50)
 y:=top+20
 drawASCII(fb,margin,y,3,"GIỚI THIỆU ỨNG DỤNG",cYellow)
 y+=53
 lines:=[]struct{heading,text string}{
  {"ỨNG DỤNG","BINANCE - THÔNG TIN THỊ TRƯỜNG"},
  {"NỀN TẢNG","TRIMUI BRICK PRO / STOCK OS"},
  {"PHIÊN BẢN",appVersion},
  {"NGƯỜI PHÁT TRIỂN","PHONGDINH91 (GITHUB)"},
  {"NGUỒN GIÁ COIN","BINANCE SPOT PUBLIC DATA"},
  {"NGUỒN TIN TỨC","24H.COM.VN"},
  {"REPOSITORY","PHONGDINH91/TRIMUI-BINANCE-OTA"},
  {"LƯU Ý","CHỈ XEM THÔNG TIN, KHÔNG ĐẶT LỆNH"},
 }
 for _,line:=range lines {
  drawASCII(fb,margin+4,y,1,line.heading,cMuted)
  drawASCII(fb,margin+4,y+19,2,line.text,cText)
  y+=55
  if y>fb.h-bottomTabsHeight(fb)-38{break}
 }
 drawBottomTabs(fb,pageSettings,"B: QUAY LẠI   L1/R1: ĐỔI TAB")
}
