package main

import (
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "net/http"
 "os"
 "strings"
)

const otaMaxZIPSize int64 = 64 << 20
const otaMaxFileSize int64 = 32 << 20

// Events are generated in the network/installer worker and drawn by the UI loop.
type otaProgressEvent struct {
 Stage string
 Done int64
 Total int64
 Message string
}

func reportOTA(progress func(otaProgressEvent), stage string, done,total int64, message string) {
 if progress!=nil {progress(otaProgressEvent{Stage:stage,Done:done,Total:total,Message:message})}
}

func otaPercent(p otaProgressEvent)(int,bool) {
 if p.Total<=0 {return 0,false}
 n:=100*p.Done/p.Total
 if n<0 {n=0};if n>100 {n=100}
 return int(n),true
}

func otaDownloadDetail(p otaProgressEvent) string {
 if p.Total<=0 {return fmt.Sprintf("%d KB DA TAI (CHUA BIET TONG)", p.Done/1024)}
 return fmt.Sprintf("%d / %d KB",p.Done/1024,p.Total/1024)
}

// Read real bytes, report progress, reject truncated/oversized downloads and verify SHA-256.
func downloadOTAWithProgress(client *http.Client,m otaManifest,dst string,progress func(otaProgressEvent))error {
 req,err:=http.NewRequest(http.MethodGet,m.URL,nil)
 if err!=nil{return err}
 if req.URL.Scheme!="https" {return errors.New("OTA chi cho phep HTTPS")}
 req.Header.Set("User-Agent","Binance-TrimUI/"+appVersion)
 resp,err:=client.Do(req)
 if err!=nil{return err}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK {
  _,_ =io.Copy(io.Discard,io.LimitReader(resp.Body,4096))
  return fmt.Errorf("OTA ZIP HTTP %d",resp.StatusCode)
 }
 if resp.ContentLength>otaMaxZIPSize {return errors.New("OTA ZIP qua lon")}
 total:=resp.ContentLength
 f,err:=os.OpenFile(dst,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0644)
 if err!=nil{return err}
 defer func(){_ = f.Close()}()
 h:=sha256.New()
 buf:=make([]byte,32*1024)
 var got int64
 reportOTA(progress,"download",0,total,"Dang ket noi...")
 for {
  n,e:=resp.Body.Read(buf)
  if n>0 {
   got+=int64(n)
   if got>otaMaxZIPSize { _=os.Remove(dst);return errors.New("OTA ZIP qua 64 MB") }
   if _,werr:=f.Write(buf[:n]);werr!=nil {_=os.Remove(dst);return werr}
   if _,werr:=h.Write(buf[:n]);werr!=nil {_=os.Remove(dst);return werr}
   reportOTA(progress,"download",got,total,"")
  }
  if e==io.EOF {break}
  if e!=nil {_=os.Remove(dst);return e}
 }
 if total>0&&got!=total {_=os.Remove(dst);return errors.New("OTA ZIP tai chua du")}
 if err:=f.Close();err!=nil {_=os.Remove(dst);return err}
 reportOTA(progress,"verify",1,1,"Dang xac minh SHA-256...")
 expected:=strings.ToLower(strings.TrimSpace(m.SHA256))
 if len(expected)!=64||hex.EncodeToString(h.Sum(nil))!=expected {
  _=os.Remove(dst)
  return errors.New("SHA-256 khong khop; da huy cai dat")
 }
 reportOTA(progress,"verify",1,1,"SHA-256 hop le")
 return nil
}

func renderOTAProgress(fb *framebuffer,p otaProgressEvent,version string) {
 w:=max(480,fb.w*76/100)
 h:=max(276,fb.h*40/100)
 if w>fb.w-24 {w=fb.w-24}
 if h>fb.h-24 {h=fb.h-24}
 x:=(fb.w-w)/2;y:=(fb.h-h)/2
 fb.rect(x-4,y-4,w+8,h+8,cYellow)
 fb.rect(x,y,w,h,cPanel)
 title:="CẬP NHẬT OTA"
 drawASCII(fb,x+max(14,(w-asciiWidth(3,title))/2),y+20,3,title,cText)
 subtitle:=appVersion+"  ->  "+version
 drawASCII(fb,x+max(14,(w-asciiWidth(2,subtitle))/2),y+65,2,subtitle,cYellow)
 stageTitle:="1/4  ĐANG TẢI BẢN CẬP NHẬT"
 switch p.Stage {
 case "verify":stageTitle="2/4  KIỂM TRA SHA-256"
 case "extract":stageTitle="3/4  GIẢI NÉN DỮ LIỆU"
 case "install":stageTitle="4/4  CÀI ĐẶT PHIÊN BẢN MỚI"
 case "done":stageTitle="HOÀN THÀNH"
 }
 drawASCII(fb,x+23,y+113,2,stageTitle,cText)
 bx:=x+25; by:=y+163; bw:=w-50; bh:=24
 fb.rect(bx,by,bw,bh,cPanel2)
 if pc,ok:=otaPercent(p);ok {
  filled:=(bw*pc)/100
  if filled>0 {fb.rect(bx,by,filled,bh,cGreen)}
  val:=fmt.Sprintf("%d%%",pc)
  drawASCII(fb,x+w-asciiWidth(2,val)-25,y+201,2,val,cYellow)
 }else{
  // Unknown HTTP Content-Length: transfer bytes are real, percentage is not guessed.
  drawASCII(fb,x+w-asciiWidth(1,"--%")-25,y+201,1,"--%",cMuted)
 }
 info:=p.Message
 if p.Stage=="download" {info=otaDownloadDetail(p)}
 if p.Stage=="extract"||p.Stage=="install" {info=fmt.Sprintf("%d/%d FILE",p.Done,p.Total)}
 if len([]rune(info))>52 {info=string([]rune(info)[:52])}
 if info!="" {drawASCII(fb,x+25,y+205,1,info,cMuted)}
 hint:="VUI LÒNG KHÔNG TẮT MÁY / RÚT THẺ"
 drawASCII(fb,x+max(12,(w-asciiWidth(1,hint))/2),y+h-30,1,hint,cYellow)
}
