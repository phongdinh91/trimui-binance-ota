package main

import (
 "debug/elf"
 "context"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "time"
)

// On-device report: only fixed system paths and their file metadata are read.
// Never include IP addresses, Wi-Fi credentials, user files, IPTV URLs,
// environment variables, network history or private configuration.
func iptvDiagnosticText(app string) string {
 lines:=[]string{
  "BINANCE "+appVersion+" - IPTV DIAGNOSTIC",
  "Captured: "+time.Now().Format("2006-01-02 15:04:05"),
  "Platform: "+runtime.GOOS+"/"+runtime.GOARCH,
 }
 if b,err:=os.ReadFile("/proc/device-tree/model");err==nil {
  model:=strings.TrimSpace(strings.ReplaceAll(string(b),"\x00"," "))
  model=strings.Join(strings.Fields(model)," ")
  if len(model)>100{model=model[:100]}
  lines=append(lines,"Device: "+model)
 }else{lines=append(lines,"Device: unavailable")}
 if b,err:=os.ReadFile("/proc/sys/kernel/osrelease");err==nil{
  lines=append(lines,"Kernel: "+strings.TrimSpace(string(b)))
 }
 for _,path:=range []string{"/mnt/SDCARD","/mnt/SDCARD/System","/mnt/SDCARD/System/bin","/mnt/SDCARD/System/lib"}{
  fi,err:=os.Stat(path)
  if err==nil && fi.IsDir(){lines=append(lines,"Directory "+path+": present")}else{
   lines=append(lines,"Directory "+path+": unavailable")
  }
 }
 if p,err:=exec.LookPath("mpv");err==nil {
  lines=append(lines,"PATH mpv: "+p)
 }else{lines=append(lines,"PATH mpv: NOT FOUND")}
 playerFound:=false
 for _,path:=range iptvPlayerPaths(app){
  fi,err:=os.Stat(path)
  if err!=nil {lines=append(lines,"MPV "+path+": missing");continue}
  state:="not executable"
  if fi.Mode().IsRegular() && fi.Mode().Perm()&0111!=0 {
   state="executable";playerFound=true
  }
  kind:="unknown format"
  if f,err:=elf.Open(path);err==nil{
   kind=fmt.Sprintf("ELF %s %s",f.FileHeader.Class,f.FileHeader.Machine)
   _=f.Close()
  }
  lines=append(lines,fmt.Sprintf("MPV %s: %s, %s, %d bytes",path,state,kind,fi.Size()))
 }
 // Safely probe only the app-local optional player: --version never opens
 // a stream, display or network but catches missing runtime linker/deps.
 localPlayer:=iptvInstalledMPVPath(app)
 if iptvExecutable(localPlayer){
  ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second)
  cmd:=exec.CommandContext(ctx,localPlayer,"--version")
  cmd.Env=iptvPlayerEnv(os.Environ(),localPlayer)
  output,err:=cmd.CombinedOutput()
  cancel()
  line:=strings.TrimSpace(string(output))
  if len(line)>250{line=line[:250]}
  line=strings.Join(strings.Fields(line)," ")
  if err!=nil {lines=append(lines,fmt.Sprintf("Bundled MPV preflight: FAIL (%v) %s",err,line))}else{
   lines=append(lines,"Bundled MPV preflight: OK "+line)
  }
 }
 if !playerFound {lines=append(lines,"RESULT: No executable MPV was found at the supported paths")}
 if playerFound {lines=append(lines,"RESULT: MPV found; output/video/audio capability is NOT verified")}
 for _,path:=range []string{"/usr/bin/tinaplayer","/usr/bin/ffplay","/mnt/SDCARD/Apps/ScreencapTK/bin/ffplay"}{
  if iptvExecutable(path){lines=append(lines,"Alternative player: "+path)}else{lines=append(lines,"Alternative player "+path+": missing")}
 }
 // Summarize the known multimedia library names without copying their content.
 for _,dir:=range []string{"/mnt/SDCARD/System/lib","/usr/lib"}{
  entries,err:=os.ReadDir(dir)
  if err!=nil{continue}
  found:=[]string{}
  for _,entry:=range entries{
   name:=entry.Name()
   if strings.HasPrefix(name,"libavcodec.so")||strings.HasPrefix(name,"libavformat.so")||strings.HasPrefix(name,"libmpv.so")||
      strings.HasPrefix(name,"libSDL2")||strings.HasPrefix(name,"libass.so"){
    if len(found)<12{found=append(found,name)}
   }
  }
  if len(found)>0{lines=append(lines,"Media libs "+dir+": "+strings.Join(found,", "))}
 }
 lines=append(lines,"Support: send this report text/screenshot to developer.")
 return strings.Join(lines,"\n")+"\n"
}
func iptvCaptureDiagnostic() string{
 report:=iptvDiagnosticText(appDir())
 if err:=os.WriteFile(filepath.Join(appDir(),"iptv-diagnostic.txt"),[]byte(report),0644);err!=nil{
  report+=fmt.Sprintf("WARNING: Cannot save local report (%v)\n",err)
 }
 return report
}
