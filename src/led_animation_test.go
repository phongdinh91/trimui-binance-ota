package main

import (
 "strings"
 "testing"
 "time"
)
func TestChaseAndRainbowFrames(t *testing.T){
 if !ledFrameAnimated(ledChase)||!ledFrameAnimated(ledRainbow) {t.Fatal("experimental modes unavailable")}
 if ledFrameAnimated(ledBlink)||ledFrameAnimated(ledGold) {t.Fatal("native modes must not use frame_hex")}
 first:=ledAnimationFrame(ledChase,0)
 second:=ledAnimationFrame(ledChase,1)
 if first==second {t.Fatal("chase frame must move")}
 if len(strings.Fields(first))!=8 {t.Fatalf("expected 8 top-bar RGB pixels, got %q",first)}
 if len(strings.Fields(ledAnimationFrame(ledRainbow,3)))!=8 {t.Fatal("rainbow must have eight pixels")}
 if ledAnimationFrame(ledRainbow,0)==ledAnimationFrame(ledRainbow,1) {t.Fatal("rainbow must rotate")}
 for _,mode:=range []int{ledRainbow,ledChase}{
  for phase:=0;phase<24;phase++ {
   pixels:=strings.Fields(ledAnimationFrame(mode,phase))
   if len(pixels)!=8{t.Fatalf("mode %d step %d: %v",mode,phase,pixels)}
   for _,p:=range pixels{
    if len(p)!=6{t.Fatalf("bad RGB value %q",p)}
   }
  }
 }
}
func TestChaseSpeedBounds(t *testing.T){
 if ledAnimationDelay(1)>=ledAnimationDelay(5){t.Fatal("fast must have shorter frame duration")}
 if ledAnimationDelay(1)<50*time.Millisecond {t.Fatal("animation speed too aggressive for sysfs driver")}
 if ledAnimationDelay(5)>time.Second {t.Fatal("slow mode unexpectedly stalled")}
}
func TestNativeAndSoftwareEffectsAreSeparate(t *testing.T){
 if settingsItemCount!=4||ledSubItemCount!=3 {t.Fatal("LED must be its own submenu")}
 if !ledDynamic(ledRainbow)||!ledDynamic(ledChase) {t.Fatal("new effects should be dynamic")}
}
