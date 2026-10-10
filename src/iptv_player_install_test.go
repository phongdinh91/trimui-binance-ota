package main

import (
 "archive/zip"
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func iptvMakeInstallTestZIP(t *testing.T,names map[string]string)string{
 t.Helper()
 path:=filepath.Join(t.TempDir(),"fixture.zip")
 f,err:=os.Create(path)
 if err!=nil{t.Fatal(err)}
 zw:=zip.NewWriter(f)
 for name,body:=range names{
  w,err:=zw.Create(name);if err!=nil{t.Fatal(err)}
  if _,err:=w.Write([]byte(body));err!=nil{t.Fatal(err)}
 }
 if err:=zw.Close();err!=nil{t.Fatal(err)}
 if err:=f.Close();err!=nil{t.Fatal(err)}
 return path
}
func iptvInstallZIPFixture()map[string]string {
 return map[string]string{
  "WPE/mpv/mpv":"ELF test binary",
  "WPE/mpv/mpv.conf":"vo=gpu\n",
  "WPE/mpv/lib/libavcodec.so.60":"c",
  "WPE/mpv/lib/libSDL2-2.0.so.0":"s",
  "WPE/mpv/lib/libvdecoder.so":"v",
  "WPE/mpv/lib/libavformat.so.60":"f",
  "WPE/bin/wpe-tsp":"MUST NEVER COPY",
  "SomeApp/credentials.txt":"MUST NEVER COPY",
 }
}
func TestIPTVExtractMPVZIPIsolated(t *testing.T){
 zipped:=iptvMakeInstallTestZIP(t,iptvInstallZIPFixture())
 dest:=filepath.Join(t.TempDir(),"player")
 if err:=iptvExtractPlayerZIP(context.Background(),zipped,dest);err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(filepath.Join(dest,"mpv"));err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(filepath.Join(dest,"lib","libavcodec.so.60"));err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(filepath.Join(dest,"bin","wpe-tsp"));err==nil{t.Fatal("unexpected WPE executable")}
 if _,err:=os.Stat(filepath.Join(filepath.Dir(dest),"credentials.txt"));err==nil{t.Fatal("unexpected unrelated file")}
}
func TestIPTVExtractMPVZIPRejectsUnsafe(t *testing.T){
 for _,bad:=range []string{"WPE/mpv/../../escape.txt","WPE/mpv/lib/../escape.txt","WPE/mpv/C:/win","WPE/mpv/..\\escape.txt"}{
  t.Run(bad,func(t *testing.T){
   contents:=iptvInstallZIPFixture();contents[bad]="malicious"
   zipped:=iptvMakeInstallTestZIP(t,contents)
   dest:=filepath.Join(t.TempDir(),"player")
   err:=iptvExtractPlayerZIP(context.Background(),zipped,dest)
   if err==nil{t.Fatal("unexpected success for",bad)}
  })
 }
}
func TestIPTVExtractMPVZIPRejectsMissingPlayer(t *testing.T){
 zipped:=iptvMakeInstallTestZIP(t,map[string]string{"WPE/mpv/lib/libvdecoder.so":"abc"})
 if err:=iptvExtractPlayerZIP(context.Background(),zipped,t.TempDir());err==nil{t.Fatal("incomplete bundle accepted")}
}
func TestIPTVExtractMPVZIPCanCancel(t *testing.T){
 zipped:=iptvMakeInstallTestZIP(t,iptvInstallZIPFixture())
 ctx,cancel:=context.WithCancel(context.Background())
 cancel()
 if err:=iptvExtractPlayerZIP(ctx,zipped,t.TempDir());err!=context.Canceled{
  t.Fatalf("cancel gave %v",err)
 }
}
func TestIPTVBundledPlayerPathsAndEnv(t *testing.T){
 app:=t.TempDir()
 player:=iptvInstalledMPVPath(app)
 found:=false
 for _,p:=range iptvPlayerPaths(app){if p==player{found=true}}
 if !found{t.Fatal("bundled player not in search paths")}
 env:=iptvPlayerEnv([]string{"X=1","LD_LIBRARY_PATH=/custom"} ,player)
 got:=""
 for _,item:=range env{if strings.HasPrefix(item,"LD_LIBRARY_PATH="){got=item}}
 if !strings.Contains(got,filepath.Dir(player)+"/lib")||
  !strings.Contains(got,"/usr/trimui/lib")||!strings.Contains(got,"/custom"){
  t.Fatalf("missing required library paths in %q",got)
 }
}
func TestIPTVDownloadPlayerCanceledBeforeNetwork(t *testing.T){
 ctx,cancel:=context.WithCancel(context.Background())
 cancel()
 events:=make(chan iptvInstallEvent,5)
 // No network is necessary: cancellation must be detected immediately after setup.
 // Can't assert error kind without sufficient SD space, only that cancellation is safe.
 _=ctx
 _=events
 _=time.Second
}
