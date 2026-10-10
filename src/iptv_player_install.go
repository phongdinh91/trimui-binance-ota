package main

import (
 "archive/zip"
 "context"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "strings"
 "syscall"
 "time"
)

const (
 iptvMPVSourceURL="https://github.com/matweew/wpe-tsp/releases/download/v0.2.0/WPE-0.2.0.zip"
 // Pinned digest of the 2026-10-10 upstream WPE v0.2.0 release ZIP.
 // Prevents silent changes to downloaded executables or libraries.
 iptvMPVSourceSHA256="97552ae45b49b2e7581f4af7b397681b5193ea7534c1e70f7a26d04f78422cdc"
 iptvMPVDownloadMax=int64(160*1024*1024)
 iptvMPVFreeRequired=uint64(230*1024*1024)
 iptvMPVEntryLimit=uint64(48*1024*1024)
 iptvMPVExtractLimit=uint64(55*1024*1024)
)

type iptvInstallEvent struct{
 Message string
 Done bool
 Err error
}
func iptvInstalledMPVPath(app string)string {
 return filepath.Join(app,"iptv-player","mpv")
}
func iptvInstallerDiskCheck(app string)error{
 var s syscall.Statfs_t
 if err:=syscall.Statfs(app,&s);err!=nil{return fmt.Errorf("không đọc được dung lượng thẻ nhớ: %w",err)}
 available:=uint64(s.Bavail)*uint64(s.Bsize)
 if available<iptvMPVFreeRequired{
  return fmt.Errorf("thẻ nhớ cần tối thiểu 230 MB trống để tải và cài trình phát (hiện còn %d MB)",available/(1024*1024))
 }
 return nil
}
// Only WPE/mpv is extracted; WebKit, WPE browser, and all other files remain
// untouched. The package is extracted into a separate app-local folder.
func iptvExtractPlayerZIP(ctx context.Context,zipPath,destination string)error{
 reader,err:=zip.OpenReader(zipPath)
 if err!=nil{return fmt.Errorf("ZIP video lỗi: %w",err)}
 defer reader.Close()
 const prefix="WPE/mpv/"
 var extracted uint64
 found:=make(map[string]bool)
 for _,f:=range reader.File{
  if ctx.Err()!=nil{return ctx.Err()}
  if !strings.HasPrefix(f.Name,prefix){continue}
  rel:=strings.TrimPrefix(f.Name,prefix)
  if rel==""{continue}
  // Must not follow any link or read paths escaping the WPE/mpv subtree.
  if strings.HasPrefix(rel,"/")||strings.ContainsRune(rel,92)||strings.Contains(rel,":")||
     filepath.IsAbs(rel)||strings.Contains("/"+rel+"/","/../")||
     strings.Contains("/"+rel+"/","/./")||filepath.Clean(rel)==".."||
     strings.HasPrefix(filepath.Clean(rel),".."+string(os.PathSeparator)){
   return fmt.Errorf("ZIP chứa đường dẫn không an toàn: %q",rel)
  }
  if f.Mode()&os.ModeSymlink!=0{return fmt.Errorf("ZIP chứa symbolic link: %q",rel)}
  if f.UncompressedSize64>iptvMPVEntryLimit{return fmt.Errorf("ZIP có file quá lớn: %s",rel)}
  if extracted+f.UncompressedSize64>iptvMPVExtractLimit{return errors.New("gói trình phát vượt giới hạn giải nén")}
  target:=filepath.Join(destination,rel)
  if f.FileInfo().IsDir(){
   if err:=os.MkdirAll(target,0755);err!=nil{return err}
   continue
  }
  if !f.Mode().IsRegular(){return fmt.Errorf("kiểu file không được phép: %q",rel)}
  if err:=os.MkdirAll(filepath.Dir(target),0755);err!=nil{return err}
  src,err:=f.Open()
  if err!=nil{return err}
  mode:=os.FileMode(0644)
  if rel=="mpv"{mode=0755}
  dst,err:=os.OpenFile(target,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,mode)
  if err!=nil{_ = src.Close();return err}
  _,copyErr:=io.Copy(dst,src)
  closeErr:=dst.Close()
  readErr:=src.Close()
  if copyErr!=nil{return copyErr}
  if closeErr!=nil{return closeErr}
  if readErr!=nil{return readErr}
  if rel=="mpv"{_ = os.Chmod(target,0755)}
  found[rel]=true
  extracted+=f.UncompressedSize64
 }
 for _,required:=range []string{"mpv","mpv.conf","lib/libavcodec.so.60","lib/libSDL2-2.0.so.0","lib/libvdecoder.so","lib/libavformat.so.60"}{
  if !found[required]{return fmt.Errorf("gói trình phát thiếu: %s",required)}
 }
 return nil
}

func iptvDownloadPlayer(ctx context.Context,app string,client *http.Client,events chan<- iptvInstallEvent) error {
 emit:=func(msg string,done bool,err error){
  select{case events<-iptvInstallEvent{Message:msg,Done:done,Err:err}:default:}
 }
 // Do not start an installation if there is inadequate free SD space.
 if err:=iptvInstallerDiskCheck(app);err!=nil{return err}
 zipPath:=filepath.Join(app,".iptv-player-download.zip")
 stage:=filepath.Join(app,".iptv-player-stage")
 target:=filepath.Join(app,"iptv-player")
 defer os.Remove(zipPath)
 defer os.RemoveAll(stage)
 if err:=os.RemoveAll(stage);err!=nil{return err}
 request,err:=http.NewRequestWithContext(ctx,http.MethodGet,iptvMPVSourceURL,nil)
 if err!=nil{return err}
 request.Header.Set("User-Agent","BINANCE-IPTV/"+appVersion)
 emit("ĐANG TẢI BỘ MPV TỪ GITHUB (~142 MB)...",false,nil)
 resp,err:=client.Do(request)
 if err!=nil{return fmt.Errorf("tải bộ MPV thất bại: %w",err)}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK{return fmt.Errorf("GitHub trả mã %d khi tải MPV",resp.StatusCode)}
 if resp.ContentLength>iptvMPVDownloadMax{return errors.New("gói tải vượt giới hạn cho phép")}
 out,err:=os.OpenFile(zipPath,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600)
 if err!=nil{return err}
 h:=sha256.New()
 total:=int64(0)
 buf:=make([]byte,128*1024)
 nextUpdate:=int64(10*1024*1024)
 for{
  if ctx.Err()!=nil{_ = out.Close();return ctx.Err()}
  n,readErr:=resp.Body.Read(buf)
  if n>0{
   total+=int64(n)
   if total>iptvMPVDownloadMax{_ = out.Close();return errors.New("tệp tải vượt giới hạn 160 MB")}
   if _,err:=out.Write(buf[:n]);err!=nil{_ = out.Close();return err}
   _,_=h.Write(buf[:n])
   if total>=nextUpdate{
    emit(fmt.Sprintf("ĐANG TẢI MPV: %d MB / ~142 MB",total/(1024*1024)),false,nil)
    nextUpdate+=10*1024*1024
   }
  }
  if readErr==io.EOF{break}
  if readErr!=nil{_ = out.Close();return readErr}
 }
 if err:=out.Close();err!=nil{return err}
 actual:=hex.EncodeToString(h.Sum(nil))
 if actual!=iptvMPVSourceSHA256{return errors.New("SHA-256 gói MPV không khớp, đã huỷ cài đặt")}
 emit("ĐÃ XÁC MINH SHA-256 / ĐANG GIẢI NÉN MPV...",false,nil)
 if err:=os.MkdirAll(stage,0755);err!=nil{return err}
 if err:=iptvExtractPlayerZIP(ctx,zipPath,stage);err!=nil{return err}
 if ctx.Err()!=nil{return ctx.Err()}
 // Stamp the source so users can audit and independently verify this third-party binary.
 notice:="Third-party optional IPTV MPV runtime\nUpstream: "+iptvMPVSourceURL+"\nSHA256: "+iptvMPVSourceSHA256+
  "\nComponents extracted: WPE/mpv/ ONLY\nSource/credits: https://github.com/matweew/wpe-tsp\n"+
  "Player uses upstream MPV/FFmpeg/Cedar dependencies. Built for TrimUI stock devices.\n"+
  "NOT verified on physical TG4040 Brick Pro.\n"
 if err:=os.WriteFile(filepath.Join(stage,"SOURCE.txt"),[]byte(notice),0644);err!=nil{return err}
 if _,err:=os.Stat(target);err==nil {
  // A previous working player is not silently replaced. User can use it or
  // remove it independently, without changing firmware or other apps.
  return errors.New("đã có bộ MPV tại iptv-player; không ghi đè")
 }
 if err:=os.Rename(stage,target);err!=nil{return fmt.Errorf("không thể hoàn tất cài đặt MPV: %w",err)}
 emit("ĐÃ CÀI BỘ MPV TRONG BINANCE / HÃY THỬ PHÁT IPTV",true,nil)
 return nil
}

func iptvInstallAsync(ctx context.Context,app string,events chan<- iptvInstallEvent) {
 client:=&http.Client{Timeout:20*time.Minute}
 err:=iptvDownloadPlayer(ctx,app,client,events)
 if err!=nil{
  msg:="CÀI MPV THẤT BẠI: "+err.Error()
  if errors.Is(err,context.Canceled){msg="ĐÃ HỦY TẢI MPV"}
  select{case events<-iptvInstallEvent{Message:msg,Done:true,Err:err}:default:}
 }
}
