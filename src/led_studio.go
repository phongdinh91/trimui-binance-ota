package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Brick Pro has several independently controlled sysfs zones. frame_hex on
// some TG4040 firmware only drives the rear strip, not the analog/shoulder
// LEDs. Never assume a 23-colour frame maps to 23 accessible physical LEDs.
var ledStudioZones = []string{"l", "r", "f1", "f2", "lr", "m", "rear"}
var ledPalette = []string{"FFB900", "00CCFF", "B46AFF", "FF3030", "00DD70", "FFFFFF", "FF67C8", "FF7B00"}

func ledPaletteAt(i int) string {
	if i < 0 || i >= len(ledPalette) { return ledPalette[0] }
	return ledPalette[i]
}
func ledNextColor(i, direction int) int {
	n := len(ledPalette)
	return ((i+direction)%n+n)%n
}

func ledStudioAvailableZones() []string {
	var zones []string
	for _, zone := range ledStudioZones {
		// Firmware may offer a combined LR channel or independent L/R.
		if zone == "lr" && ledNodeExists("effect_rgb_hex_l") && ledNodeExists("effect_rgb_hex_r") {
			continue
		}
		if ledNodeExists("effect_"+zone) && ledNodeExists("effect_rgb_hex_"+zone) {
			zones=append(zones,zone)
		}
	}
	return zones
}
func ledNodeExists(name string) bool {
	_, err := os.Stat(ledDriverPath(name))
	return err == nil
}
func ledStudioCheck() ([]string,error) {
	for _, node := range []string{"effect_enable","max_scale","effect_m","effect_rgb_hex_m"} {
		if !ledNodeExists(node) { return nil,fmt.Errorf("firmware thiếu LED node %s",node) }
	}
	zones:=ledStudioAvailableZones()
	if len(zones)<2 {
		return nil,errors.New("driver chỉ công bố 1 vùng LED; cần kiểm tra /sys/class/led_anim/help trước khi bật hiệu ứng toàn máy")
	}
	return zones,nil
}
func hexRGB(c string) (int,int,int) {
	if len(c)!=6 { return 0,0,0 }
	n,e:=strconv.ParseUint(c,16,24)
	if e!=nil { return 0,0,0 }
	return int(n>>16)&255,int(n>>8)&255,int(n)&255
}
func ledBlend(a,b string,ratio float64) string {
	if ratio<0{ratio=0};if ratio>1{ratio=1}
	ar,ag,ab:=hexRGB(a)
	br,bg,bb:=hexRGB(b)
	mix:=func(a,b int) int { return int(math.Round(float64(a)*(1-ratio)+float64(b)*ratio)) }
	return fmt.Sprintf("%02X%02X%02X",mix(ar,br),mix(ag,bg),mix(ab,bb))
}
func ledScale(c string, ratio float64) string { return ledBlend("000000",c,ratio) }

func ledStudioColor(mode,phase,index,count,battery,actionAge int, primary,secondary string) string {
	if count<1{return "000000"}
	rainbow:=[]string{"FF3030","FF9900","FFE000","30FF30","00E8CF","205EFF","A230FF","FF40C0"}
	switch mode {
	case ledBreathing:
		wave:=float64(phase%24)/12
		if wave>1{wave=2-wave}
		return ledScale(primary,0.12+wave*0.88)
	case ledBlink:
		if phase%2==0{return primary}
		return "000000"
	case ledBattery:
		switch {
		case battery<0: return "000000"
		case battery<20:
			if phase%2==1{return "000000"}
			return "FF2020"
		case battery<=50:return "FFB900"
		default:return "00DD70"
		}
	case ledDualTone:
		if index<count/2{return primary}
		return secondary
	case ledColorCycle:
		return rainbow[(phase/3)%len(rainbow)]
	case ledAmbient:
		return ledScale(primary,0.30)
	case ledRainbow:
		return rainbow[(index*2+phase)%len(rainbow)]
	case ledChase:
		head:=phase%count
		if index==head{return primary}
		if index==(head+count-1)%count{return ledScale(primary,0.22)}
		return "000000"
	case ledAction:
		if actionAge>=0 && actionAge<5 {
			return ledBlend(primary,secondary,1-float64(actionAge)/5)
		}
		return ledScale(primary,0.12)
	default:
		return primary
	}
}
func ledStudioFrame(mode,phase,battery,actionAge int, zones []string, primary,secondary string) map[string]string {
	colors:=make(map[string]string,len(zones))
	for i,zone:=range zones {
		if mode==ledDualTone {
            switch zone {
            case "l","f1": colors[zone]=primary
            case "r","f2": colors[zone]=secondary
            default: colors[zone]=ledBlend(primary,secondary,0.5)
            }
        } else {
            colors[zone]=ledStudioColor(mode,phase,i,len(zones),battery,actionAge,primary,secondary)
        }
	}
	return colors
}
func ledBatteryPercent() (int,error) {
	power:="/sys/class/power_supply"
	entries,err:=os.ReadDir(power)
	if err!=nil{return -1,err}
	for _,entry:=range entries {
		root:=filepath.Join(power,entry.Name())
		t,err:=os.ReadFile(filepath.Join(root,"type"))
		if err!=nil||!strings.EqualFold(strings.TrimSpace(string(t)),"Battery"){continue}
		b,err:=os.ReadFile(filepath.Join(root,"capacity"))
		if err!=nil{continue}
		value,err:=strconv.Atoi(strings.TrimSpace(string(b)))
		if err==nil&&value>=0&&value<=100 {return value,nil}
	}
	return -1,errors.New("không đọc được mức pin từ /sys/class/power_supply")
}
// The TG4040 driver has two gates: "enable" is the hardware master,
// while "effect_enable" selects the zone-based engine. The v0.38 code
// never enabled the master. On compatible firmware, setting effect_*
// AFTER writing its colour is also necessary to trigger a visible change.
func ledStudioPrepare(brightness int,zones []string) error {
	if ledNodeExists("enable") {
		if err:=writeLEDNode("enable","1");err!=nil{
			return fmt.Errorf("không bật được công tắc LED tổng: %w",err)
		}
	}
	if err:=writeLEDNode("effect_enable","1");err!=nil{return err}
	for _,node:=range []string{"max_scale","max_scale_lr","max_scale_f1f2","max_scale_rear"} {
		if err:=writeLEDOptional(node,strconv.Itoa(brightness));err!=nil{return err}
	}
	return nil
}

// Setting colour alone is not enough on some Stock OS revisions.
// Re-arm the native static effect AFTER the new colour is written.
// No frame_hex writes (which can deadlock on some firmware).
func ledStudioApplyZone(zone,color string,write func(string,string)error) error {
	if len(color)!=6{return fmt.Errorf("màu LED vùng %s không hợp lệ",zone)}
	if err:=write("effect_rgb_hex_"+zone,color+" ");err!=nil{return err}
	if err:=write("effect_"+zone,"0");err!=nil{return err}
	if err:=write("effect_"+zone,"4");err!=nil{return err}
	return nil
}
func ledStudioWriteFrame(frame map[string]string,zones []string)error{
	for _,zone:=range zones{
		color,ok:=frame[zone]
		if !ok{return fmt.Errorf("khung LED thiếu vùng %s",zone)}
		if err:=ledStudioApplyZone(zone,color,writeLEDNode);err!=nil{return err}
	}
	return nil
}

func ledStudioRun(mode,brightness,speed,colorA,colorB int, feedback interface{ Write([]byte)(int,error) }, actions <-chan struct{}, stop <-chan os.Signal) error {
	fail:=func(err error)error{_,_=fmt.Fprintln(feedback,"ERROR:",err);return err}
	if speed<1||speed>ledSpeedLevels{return fail(errors.New("tốc độ LED không hợp lệ"))}
	zones,err:=ledStudioCheck()
	if err!=nil{return fail(err)}
	if mode==ledBattery {
		if _,err=ledBatteryPercent();err!=nil{return fail(err)}
	}
	if err=ledStudioPrepare(brightness,zones);err!=nil{return fail(err)}
	phase,battery,actionAge:=0,-1,100
	if mode==ledBattery {battery,_=ledBatteryPercent()}
	primary,secondary:=ledPaletteAt(colorA),ledPaletteAt(colorB)
	last:=make(map[string]string,len(zones))
	render:=func()error{
		frame:=ledStudioFrame(mode,phase,battery,actionAge,zones,primary,secondary)
		for _,zone:=range zones {
			color:=frame[zone]
			if last[zone]==color{continue}
			if err:=ledStudioApplyZone(zone,color,writeLEDNode);err!=nil{return err}
			last[zone]=color
		}
		return nil
	}
	if err=render();err!=nil{return fail(err)}
	_,_=fmt.Fprintln(feedback,"READY")
    if mode==ledDualTone||mode==ledAmbient {<-stop;return nil}
	ticker:=time.NewTicker(ledAnimationDelay(speed))
	defer ticker.Stop()
	for {
		select {
		case <-stop:return nil
		case <-actions:actionAge=0
		case <-ticker.C:
			phase++
			if actionAge<100{actionAge++}
			if mode==ledBattery && phase%10==0{
				if v,e:=ledBatteryPercent();e==nil{battery=v}
			}
		}
		if err:=render();err!=nil{return err}
	}
}
