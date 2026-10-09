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

// Experimental animations control the eight top-bar LEDs only.
// On supported Brick-family drivers, frame_hex requires effect_enable 1 -> 0
// BETWEEN frames. Repeating frame_hex without that toggle can block in kernel.
func ledFrameAnimated(mode int) bool {return mode==ledRainbow||mode==ledChase}

var ledRainbowColors=[8]string{"FF2020","FF9900","FFE000","30FF30","00E8CF","205EFF","A230FF","FF40C0"}

func ledAnimationFrame(mode,phase int) string {
 var pixels [8]string
 for i:=range pixels {pixels[i]="000000"}
 if mode==ledRainbow{
  for i:=range pixels {pixels[i]=ledRainbowColors[(i+phase)%8]}
 }else if mode==ledChase {
  h:=phase%8
  if h<0{h+=8}
  pixels[h]="FFB900"
  pixels[(h+7)%8]="704000"
  pixels[(h+6)%8]="281200"
 }
 return strings.Join(pixels[:]," ")+" "
}
func ledAnimationDelay(speed int) time.Duration {
 switch speed {
 case 1: return 85*time.Millisecond
 case 2: return 125*time.Millisecond
 case 3: return 180*time.Millisecond
 case 4: return 270*time.Millisecond
 default: return 400*time.Millisecond
 }
}
func checkLEDFrameSupport()error{
 for _,node:=range []string{"frame_hex","effect_enable","max_scale","effect_rgb_hex_m","effect_m"} {
  path:=ledDriverPath(node)
  f,e:=os.OpenFile(path,os.O_WRONLY,0)
  if e!=nil {return fmt.Errorf("không thể điều khiển LED %s: %w",node,e)}
  _=f.Close()
 }
 return nil
}
type ledAnimationProcess struct {cmd *exec.Cmd}
func (p *ledAnimationProcess) stop()error{
 if p==nil||p.cmd==nil||p.cmd.Process==nil{return nil}
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
 if s.LEDBrightness<ledBrightnessMin||s.LEDBrightness>ledBrightnessMax||s.LEDSpeed<1||s.LEDSpeed>ledSpeedLevels{
  return nil,errors.New("độ sáng hoặc tốc độ không hợp lệ")
 }
 if e:=checkLEDFrameSupport();e!=nil{return nil,e}
 exe,e:=os.Executable();if e!=nil{return nil,e}
 parent,child,e:=os.Pipe();if e!=nil{return nil,e}
 defer parent.Close()
 cmd:=exec.Command(exe,"--led-worker",strconv.Itoa(s.LEDMode),strconv.Itoa(s.LEDBrightness),strconv.Itoa(s.LEDSpeed))
 cmd.ExtraFiles=[]*os.File{child}
 cmd.SysProcAttr=&syscall.SysProcAttr{Pdeathsig:syscall.SIGTERM}
 if e=cmd.Start();e!=nil{_ = child.Close();return nil,e}
 _=child.Close()
 proc:=&ledAnimationProcess{cmd:cmd}
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
func runLEDAnimationWorker(mode,brightness,speed int,feedback io.Writer)error{
 sendErr:=func(e error)error{fmt.Fprintln(feedback,"ERROR:",e.Error());return e}
 if !ledFrameAnimated(mode){return sendErr(errors.New("chế độ hoạt ảnh không hợp lệ"))}
 if brightness<ledBrightnessMin||brightness>ledBrightnessMax||speed<1||speed>ledSpeedLevels{
  return sendErr(errors.New("mức LED vượt giới hạn"))
 }
 if e:=checkLEDFrameSupport();e!=nil{return sendErr(e)}
 signalCh:=make(chan os.Signal,1)
 signal.Notify(signalCh,syscall.SIGTERM,syscall.SIGINT)
 defer signal.Stop(signalCh)
 defer func(){_ = writeLEDNode("effect_enable","1")}()
 if e:=writeLEDNode("max_scale",strconv.Itoa(brightness));e!=nil{return sendErr(e)}
 if e:=writeLEDNode("effect_rgb_hex_m","000000 ");e!=nil{return sendErr(e)}
 if e:=writeLEDNode("effect_m","4");e!=nil{return sendErr(e)}
 if e:=writeLEDNode("effect_enable","0");e!=nil{return sendErr(e)}
 time.Sleep(160*time.Millisecond)
 if e:=writeLEDNode("frame_hex",ledAnimationFrame(mode,0));e!=nil{return sendErr(e)}
 fmt.Fprintln(feedback,"READY")
 delay:=ledAnimationDelay(speed)
 for frame:=1;;frame++{
  select{case <-signalCh:return nil;default:}
  select{case <-signalCh:return nil;case <-time.After(delay):}
  // Tested Brick workaround: re-enable then disable built-in effects
  // before each subsequent frame to release the driver's write lock.
  if e:=writeLEDNode("effect_enable","1");e!=nil{return e}
  if e:=writeLEDNode("effect_enable","0");e!=nil{return e}
  if e:=writeLEDNode("frame_hex",ledAnimationFrame(mode,frame));e!=nil{return e}
 }
}
func handleLEDWorkerArgs(args []string)bool{
 if len(args)!=4 || args[0]!="--led-worker"{return false}
 mode,e1:=strconv.Atoi(args[1]);brightness,e2:=strconv.Atoi(args[2]);speed,e3:=strconv.Atoi(args[3])
 if e1!=nil||e2!=nil||e3!=nil {return true}
 ready:=os.NewFile(uintptr(3),"led-worker-ready")
 if ready==nil{return true}
 defer ready.Close()
 _=runLEDAnimationWorker(mode,brightness,speed,ready)
 return true
}
