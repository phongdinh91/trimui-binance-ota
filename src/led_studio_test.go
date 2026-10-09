package main

import (
	"strings"
	"testing"
)

func TestLEDStudioNineEffectFamilies(t *testing.T) {
	modes := []int{ledGold,ledBreathing,ledBattery,ledDualTone,ledColorCycle,ledAmbient,ledRainbow,ledChase,ledAction}
	if len(modes)!=9{t.Fatal("nine styles")}
	zones:=[]string{"l","r","f1","f2","m","rear"}
	for _,mode:=range modes {
		frame:=ledStudioFrame(mode,0,90,100,zones,"FFB900","00CCFF")
		if len(frame)!=len(zones){t.Fatalf("mode %d missing zones",mode)}
		for _,zone:=range zones {
			rgb:=frame[zone]
			if len(rgb)!=6 || strings.Contains(rgb," "){t.Fatalf("mode %d zone %s: invalid %q",mode,zone,rgb)}
		}
	}
	if ledStudioFrame(ledRainbow,0,80,100,zones,"FFB900","00CCFF")["l"] ==
		ledStudioFrame(ledRainbow,0,80,100,zones,"FFB900","00CCFF")["r"] {
		t.Fatal("rainbow must span multiple physical zones")
	}
}
func TestLEDStudioChaseVisitsAllZones(t *testing.T){
	zones:=[]string{"l","r","f1","f2","m","rear"}
	for i:=range zones {
		frame:=ledStudioFrame(ledChase,i,80,100,zones,"FFB900","00CCFF")
		if frame[zones[i]]!="FFB900"{t.Fatalf("chase never highlights zone %s",zones[i])}
	}
}
func TestLEDBatteryAndAction(t *testing.T){
	if ledStudioColor(ledBattery,0,0,6,15,100,"FFB900","00CCFF")!="FF2020"{t.Fatal("low battery warning")}
	if ledStudioColor(ledBattery,0,0,6,40,100,"FFB900","00CCFF")!="FFB900"{t.Fatal("mid battery colour")}
	if ledStudioColor(ledBattery,0,0,6,85,100,"FFB900","00CCFF")!="00DD70"{t.Fatal("charged battery colour")}
	if ledStudioColor(ledBattery,0,0,6,-1,100,"FFB900","00CCFF")!="000000"{t.Fatal("unavailable battery must not invent charge")}
	active:=ledStudioColor(ledAction,1,0,6,85,0,"FFB900","00CCFF")
	idle:=ledStudioColor(ledAction,1,0,6,85,100,"FFB900","00CCFF")
	if active==idle{t.Fatal("button press must change output")}
}
func TestLEDPaletteAndGradient(t *testing.T){
	if ledNextColor(0,-1)!=len(ledPalette)-1 || ledNextColor(len(ledPalette)-1,1)!=0 {
		t.Fatal("palette wrap")
	}
	if got:=ledBlend("000000","FFFFFF",0.5);got!="808080"{t.Fatalf("blend midpoint: %s",got)}
	if got:=ledStudioColor(ledDualTone,0,0,6,80,100,"FFB900","00CCFF");got!="FFB900"{t.Fatal("left tone")}
	if got:=ledStudioColor(ledDualTone,0,5,6,80,100,"FFB900","00CCFF");got!="00CCFF"{t.Fatal("right tone")}
}
func TestNewModesPersistWithoutReindexing(t *testing.T) {
	if ledSystem!=0||ledGold!=2||ledRainbow!=7||ledChase!=8||ledGreen!=10{t.Fatal("do not break saved old LED mode values")}
	if ledBattery!=11||ledAction!=15||len(ledModeNames)!=ledModeCount{t.Fatal("new mode table mismatch")}
	for _,mode:=range []int{ledBreathing,ledBlink,ledRainbow,ledChase,ledBattery,ledDualTone,ledColorCycle,ledAmbient,ledAction}{
		if !ledFrameAnimated(mode){t.Fatalf("studio worker missing mode %d",mode)}
	}
}
