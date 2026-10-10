package main

import (
 "bufio"
 "errors"
 "fmt"
 "io"
 "os"
 "os/exec"
 "os/signal"
 "strconv"
 "strings"
 "syscall"
 "time"
)

// Experimental animations address all device zones. Brick Pro (TG4040) has
// a different LED topology from original Brick; sending only eight pixels
// addressed a single strip and left the other zones unchanged.
// frame_hex requires effect_enable 1 -> 0 BETWEEN writes, or the kernel
// driver can block on the next write. Hardware output remains experimental.
func ledFrameAnimated(mode int) bool {return mode==ledBreathing||mode==ledBlink||mode==ledRainbow||mode==ledChase||mode==ledBattery||mode==ledDualTone||mode==ledColorCycle||mode==ledAmbient||mode==ledAction}

var ledRainbowColors=[8]string{"FF2020","FF9900","FFE000","30FF30","00E8CF","205EFF","A230FF","FF40C0"}

const (
 ledOriginalBrickPixels=14
 ledBrickProPixels=23
)
// Brick Pro exposes an additional REAR zone absent from original Brick.
// Check the exact model when available and the rear brightness node as fallback.
func ledAnimationPixelCount() int {
 for _,file:=range []string{"/proc/device-tree/model","/proc/device-tree/compatible"}{
  model,e:=os.ReadFile(file)
  if e==nil {
   lower:=strings.ToLower(strings.ReplaceAll(string(model),"\x00"," "))
   if strings.Contains(lower,"tg4040")||strings.Contains(lower,"brick pro")||strings.Contains(lower,"brickpro"){
    return ledBrickProPixels
   }
  }
 }
 if _,e:=os.Stat(ledDriverPath("max_scale_rear"));e==nil{return ledBrickProPixels}
 return ledOriginalBrickPixels
}
func ledAnimationFrameFor(mode,phase,pixelCount int)string{
 if pixelCount!=ledOriginalBrickPixels&&pixelCount!=ledBrickProPixels{return ""}
 pixels:=make([]string,pixelCount)
 for i:=range pixels {pixels[i]="000000"}
 if mode==ledRainbow{
  for i:=range pixels {pixels[i]=ledRainbowColors[(i+phase)%len(ledRainbowColors)]}
 }else if mode==ledChase{
  head:=phase%pixelCount
  if head<0{head+=pixelCount}
  pixels[head]="FFB900"
  pixels[(head+pixelCount-1)%pixelCount]="704000"
  pixels[(head+pixelCount-2)%pixelCount]="281200"
 }
 return strings.Join(pixels," ")+" "
}
func ledAnimationFrame(mode,phase int)string{
 return ledAnimationFrameFor(mode,phase,ledAnimationPixelCount())
}
func ledAnimationDelay(speed int) time.Duration {
 switch speed {
 case 1: return 160*time.Millisecond
 case 2: return 220*time.Millisecond
 case 3: return 320*time.Millisecond
 case 4: return 480*time.Millisecond
 default: return 700*time.Millisecond
 }
}
func checkLEDFrameSupport() error { _, err := ledStudioCheck(); return err }

// Keep every available brightness zone active; clearing zone effect colours
// prevents visible flashes during effect_enable toggles between frames.
func prepareLEDZones(brightness int)error{
 value:=strconv.Itoa(brightness)
 for _,node:=range []string{"max_scale","max_scale_lr","max_scale_f1f2","max_scale_rear"}{
  if node=="max_scale"{if e:=writeLEDNode(node,value);e!=nil{return e}
  }else if e:=writeLEDOptional(node,value);e!=nil{return e}
 }
 for _,zone:=range []string{"m","f1","f2","lr","rear","l","r"}{
  if e:=writeLEDOptional("effect_rgb_hex_"+zone,"000000 ");e!=nil{return e}
  if e:=writeLEDOptional("effect_"+zone,"4");e!=nil{return e}
 }
 return nil
}
type ledAnimationProcess struct {cmd *exec.Cmd; actionPipe *os.File}
func (p *ledAnimationProcess) pulse(){ if p!=nil&&p.actionPipe!=nil{_,_=p.actionPipe.Write([]byte("1\n"))} }
func (p *ledAnimationProcess) stop()error{
 if p==nil||p.cmd==nil||p.cmd.Process==nil{return nil}
 if p.actionPipe!=nil { _=p.actionPipe.Close();p.actionPipe=nil }
 _=p.cmd.Process.Signal(syscall.SIGTERM)
 done:=make(chan error,1)
 go func(){done<-p.cmd.Wait()}()
 select {
 case <-done:
  _=writeLEDNode("effect_enable","1")
  return nil
 case <-time.After(1400*time.Millisecond):
  _=p.cmd.Process.Kill()
  select{
  case <-done:
   _=writeLEDNode("effect_enable","1")
   return nil
  case <-time.After(700*time.Millisecond):
   return errors.New("driver LED không dừng kịp; cần khởi động lại máy trước khi đổi hiệu ứng")
  }
 }
}
func startLEDAnimation(s settingsFile)(*ledAnimationProcess,error){
 if !ledFrameAnimated(s.LEDMode){return nil,errors.New("không phải hiệu ứng chạy LED")}
 if s.LEDBrightness<ledBrightnessMin||s.LEDBrightness>ledBrightnessMax||s.LEDSpeed<1||s.LEDSpeed>ledSpeedLevels||s.LEDPrimary<0||s.LEDPrimary>=len(ledPalette)||s.LEDSecondary<0||s.LEDSecondary>=len(ledPalette){
  return nil,errors.New("độ sáng hoặc tốc độ không hợp lệ")
 }
 if e:=checkLEDFrameSupport();e!=nil{return nil,e}
 exe,e:=os.Executable();if e!=nil{return nil,e}
 parent,child,e:=os.Pipe();if e!=nil{return nil,e}
 defer parent.Close()
 actionReader,actionWriter,e:=os.Pipe();if e!=nil{_ = child.Close();return nil,e}
 cmd:=exec.Command(exe,"--led-worker",strconv.Itoa(s.LEDMode),strconv.Itoa(s.LEDBrightness),strconv.Itoa(s.LEDSpeed),strconv.Itoa(s.LEDPrimary),strconv.Itoa(s.LEDSecondary))
 cmd.ExtraFiles=[]*os.File{child,actionReader}
 cmd.SysProcAttr=&syscall.SysProcAttr{Pdeathsig:syscall.SIGTERM}
 if e=cmd.Start();e!=nil{_ = child.Close();_ = actionReader.Close();_ = actionWriter.Close();return nil,e}
 _=child.Close()
 _=actionReader.Close()
 proc:=&ledAnimationProcess{cmd:cmd,actionPipe:actionWriter}
 ready:=make(chan string,1)
 go func(){
  line,readErr:=bufio.NewReader(parent).ReadString('\n')
  if readErr!=nil&&line==""{ready<-"ERROR: Không nhận phản hồi từ LED"}else{ready<-strings.TrimSpace(line)}
 }()
 select {
 case line:=<-ready:
  if line=="READY"{return proc,nil}
  _=proc.stop()
  return nil,errors.New(strings.TrimPrefix(line,"ERROR:"))
 case <-time.After(3*time.Second):
  _=proc.stop()
  return nil,errors.New("driver LED không phản hồi sau 3 giây")
 }
}
func runLEDAnimationWorker(mode,brightness,speed,colorA,colorB int,feedback io.Writer) error {
 sendErr:=func(err error)error{fmt.Fprintln(feedback,"ERROR:",err.Error());return err}
 if !ledFrameAnimated(mode){return sendErr(errors.New("chế độ hoạt ảnh không hợp lệ"))}
 if brightness<ledBrightnessMin||brightness>ledBrightnessMax||speed<1||speed>ledSpeedLevels{
  return sendErr(errors.New("mức LED vượt giới hạn"))
 }
 signalCh:=make(chan os.Signal,1)
 signal.Notify(signalCh,syscall.SIGTERM,syscall.SIGINT)
 defer signal.Stop(signalCh)
 defer func(){_ = writeLEDNode("effect_enable","1")}()
 actions:=make(chan struct{},1)
 input:=os.NewFile(uintptr(4),"led-action-events")
 if input!=nil {
  defer input.Close()
  go func(){
   scanner:=bufio.NewScanner(input)
   for scanner.Scan(){ select { case actions<-struct{}{}:default: } }
  }()
 }
 return ledStudioRun(mode,brightness,speed,colorA,colorB,feedback,actions,signalCh)
}
func handleLEDWorkerArgs(args []string) bool {
 if len(args)!=6 || args[0]!="--led-worker"{return false}
 mode,e1:=strconv.Atoi(args[1])
 brightness,e2:=strconv.Atoi(args[2])
 speed,e3:=strconv.Atoi(args[3])
 primary,e4:=strconv.Atoi(args[4])
 secondary,e5:=strconv.Atoi(args[5])
 if e1!=nil||e2!=nil||e3!=nil||e4!=nil||e5!=nil{return true}
 ready:=os.NewFile(uintptr(3),"led-worker-ready")
 if ready==nil{return true}
 defer ready.Close()
 if err:=runLEDAnimationWorker(mode,brightness,speed,primary,secondary,ready);err!=nil{fmt.Fprintln(os.Stderr,"LED worker stopped:",err)}
 return true
}
