package main

import (
 "strings"
 "testing"
 "time"
)

func TestFullDeviceRainbowAndChaseFrames(t *testing.T){
 if !ledFrameAnimated(ledChase)||!ledFrameAnimated(ledRainbow){t.Fatal("effects unavailable")}
 if ledFrameAnimated(ledBlink)||ledFrameAnimated(ledGold){t.Fatal("native modes must not use frame_hex")}
 for _,count:=range []int{ledOriginalBrickPixels,ledBrickProPixels}{
  for _,mode:=range []int{ledRainbow,ledChase}{
   first:=ledAnimationFrameFor(mode,0,count)
   second:=ledAnimationFrameFor(mode,1,count)
   if first==second{t.Fatalf("mode %d device %d: frame must move",mode,count)}
   for phase:=0;phase<count*2;phase++{
    frame:=ledAnimationFrameFor(mode,phase,count)
    colors:=strings.Fields(frame)
    if len(colors)!=count{t.Fatalf("mode %d frame %d: expected %d RGB pixels, got %d",mode,phase,count,len(colors))}
    if !strings.HasSuffix(frame," "){t.Fatal("sysfs frames require trailing space")}
    for _,c:=range colors{
     if len(c)!=6{t.Fatalf("invalid RGB entry %q",c)}
    }
   }
  }
 }
 if ledAnimationFrameFor(ledRainbow,0,8)!=""{t.Fatal("8-pixel partial frame must be rejected")}
}
func TestChaseIlluminatesAdditionalZones(t *testing.T){
 for _,count:=range []int{ledOriginalBrickPixels,ledBrickProPixels}{
  // A full chase must eventually reach *every* position, not just the first eight.
  seen:=make([]bool,count)
  for phase:=0;phase<count;phase++{
   colors:=strings.Fields(ledAnimationFrameFor(ledChase,phase,count))
   for i,color:=range colors {if color=="FFB900"{seen[i]=true}}
  }
  for i,v:=range seen {if !v{t.Fatalf("pixel %d/%d never reached",i+1,count)}}
 }
}
func TestLEDSpeedDelay(t *testing.T){
 if ledAnimationDelay(1)>=ledAnimationDelay(5){t.Fatal("level 1 must be fastest")}
 if ledAnimationDelay(1)<50*time.Millisecond{t.Fatal("animation speed too aggressive for sysfs driver")}
 if ledAnimationDelay(5)>time.Second{t.Fatal("slow mode stalled")}
}
func TestNativeAndSoftwareEffectsAreSeparate(t *testing.T){
 if settingsItemCount!=4||ledSubItemCount!=3{t.Fatal("LED must stay a Settings submenu")}
 if !ledDynamic(ledRainbow)||!ledDynamic(ledChase){t.Fatal("animation modes must be dynamic")}
}
